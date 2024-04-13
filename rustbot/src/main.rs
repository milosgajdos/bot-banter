use async_nats::jetstream::{
    self,
    consumer::{self, pull},
    stream,
};
use bytes::{Bytes, BytesMut};
use ollama_rs::{generation::completion::request::GenerationRequest, Ollama};
use tokio::{
    self, signal,
    sync::mpsc::{self, Receiver, Sender},
};
use tokio_stream::StreamExt;

mod history;

const NATS_URL: &str = "nats://localhost:4222";
const HIST_SIZE: usize = 50;
const MODEL_NAME: &str = "llama2:latest";
const STREAM_NAME: &str = "banter";
const BOT_NAME: &str = "rustbot";
const BOT_SUB_SUBJECT: &str = "rust";
const BOT_PUB_SUBJECT: &str = "go";
const BOT_PUB_NAME: &str = "gobot";

pub type Result<T> = std::result::Result<T, Box<dyn std::error::Error + Send + Sync>>;

async fn llm_stream(
    llm: Ollama,
    mut prompts: Receiver<String>,
    chunks: Sender<Bytes>,
) -> Result<()> {
    println!("launched LLM stream");
    use history::History;
    let mut history = History::new(HIST_SIZE);

    while let Some(prompt) = prompts.recv().await {
        history.add(prompt.clone());
        let mut stream = llm
            .generate_stream(GenerationRequest::new(
                MODEL_NAME.to_owned(),
                history.string(),
            ))
            .await?;

        while let Some(res) = stream.next().await {
            let responses = res?;
            for resp in responses {
                chunks.send(Bytes::from(resp.response)).await?;
            }
        }
    }

    Ok(())
}

async fn jetstream_read(
    cons: consumer::Consumer<pull::Config>,
    prompts: Sender<String>,
) -> Result<()> {
    println!("launched JetStream Reader");
    let mut messages = cons.messages().await?;
    while let Some(Ok(message)) = messages.next().await {
        println!("\n[{}]: {:?}", BOT_PUB_NAME, message.payload.to_owned());
        message.ack().await?;
        // NOTE: maybe we can send an empty string of the conversion fails?
        let prompt = String::from_utf8(message.payload.to_vec())?;
        prompts.send(prompt).await?;
    }

    Ok(())
}

async fn jetstream_write(js: jetstream::Context, mut chunks: Receiver<Bytes>) -> Result<()> {
    println!("launched JetStream Writer");
    let mut b = BytesMut::new();
    while let Some(chunk) = chunks.recv().await {
        if chunk.is_empty() {
            let msg = String::from_utf8(b.to_vec()).unwrap();
            println!("\n[{}]: {}", BOT_NAME, msg);
            js.publish(BOT_PUB_SUBJECT.to_string(), b.clone().freeze())
                .await?;
            b.clear();
            continue;
        }
        b.extend_from_slice(&chunk);
    }

    Ok(())
}

#[tokio::main]
async fn main() -> Result<()> {
    let ollama = Ollama::default();

    let nats_url = std::env::var("NATS_URL").unwrap_or_else(|_| NATS_URL.to_string());

    let client = async_nats::connect(nats_url).await?;

    let js = jetstream::new(client);

    let stream = js
        .get_or_create_stream(stream::Config {
            name: STREAM_NAME.to_string(),
            ..Default::default()
        })
        .await?;

    let cons = stream
        .create_consumer(pull::Config {
            durable_name: Some(BOT_NAME.to_string()),
            filter_subject: BOT_SUB_SUBJECT.to_string(),
            ..Default::default()
        })
        .await?;

    let (prompts_tx, prompts_rx) = mpsc::channel::<String>(32);
    let (chunks_tx, chunks_rx) = mpsc::channel::<Bytes>(32);

    println!("launching {} workers", BOT_NAME);

    let llm_stream_task = tokio::spawn(llm_stream(ollama, prompts_rx, chunks_tx));
    let jetstream_write_task = tokio::spawn(jetstream_write(js, chunks_rx));
    let jetstream_read_task = tokio::spawn(jetstream_read(cons, prompts_tx));

    tokio::pin!(llm_stream_task, jetstream_write_task, jetstream_read_task);

    tokio::select! {
        result = &mut llm_stream_task => {
            if let Err(e) = result {
                eprintln!("shutting down, llm_stream encountered an error: {}", e);
                jetstream_write_task.abort();
                jetstream_read_task.abort();
                return Err(e.into());
            }
        }
        result = &mut jetstream_write_task => {
            if let Err(e) = result {
                eprintln!("shutting down, jetstream_write encountered an error: {}", e);
                llm_stream_task.abort();
                jetstream_read_task.abort();
                return Err(e.into());
            }
        }
        result = &mut jetstream_read_task => {
            if let Err(e) = result {
                eprintln!("shutting down, jetstream_read encountered an error: {}", e);
                llm_stream_task.abort();
                jetstream_write_task.abort();
                return Err(e.into());
            }
        }
        _ = signal::ctrl_c() => {
            println!("shutting down, received SIGINT signal...");
            llm_stream_task.abort();
            jetstream_write_task.abort();
            jetstream_read_task.abort();
            return Ok(());
        }
    }

    Ok(())
}
