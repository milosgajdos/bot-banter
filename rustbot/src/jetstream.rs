use crate::prelude::*;
use async_nats::jetstream::{
    self,
    consumer::{pull, Consumer},
    stream,
};
use bytes::{Bytes, BytesMut};
use tokio::{
    self,
    sync::mpsc::{Receiver, Sender},
};
use tokio_stream::StreamExt;

#[derive(Clone, Debug)]
pub struct Config {
    pub nats_url: String,
    pub durable_name: String,
    pub stream_name: String,
    pub pub_subject: String,
    pub sub_subject: String,
}

impl Default for Config {
    fn default() -> Self {
        Config {
            nats_url: std::env::var("NATS_URL").unwrap_or_else(|_| NATS_DEFAULT_URL.to_string()),
            durable_name: BOT_NAME.to_string(),
            stream_name: STREAM_NAME.to_string(),
            pub_subject: BOT_PUB_SUBJECT.to_string(),
            sub_subject: BOT_SUB_SUBJECT.to_string(),
        }
    }
}

pub struct Writer {
    tx: jetstream::Context,
    subject: String,
}

#[allow(unused)]
pub struct Reader {
    rx: Consumer<pull::Config>,
    subject: String,
}

pub async fn new(c: Config) -> Result<(Writer, Reader)> {
    let client = async_nats::connect(c.nats_url).await?;
    let js = jetstream::new(client);

    let stream = js
        .get_or_create_stream(stream::Config {
            name: c.stream_name,
            ..Default::default()
        })
        .await?;

    let cons = stream
        .create_consumer(pull::Config {
            durable_name: Some(c.durable_name.clone()),
            filter_subject: c.sub_subject.clone(),
            ..Default::default()
        })
        .await?;

    Ok((
        Writer {
            tx: js,
            subject: c.pub_subject.clone(),
        },
        Reader {
            rx: cons,
            subject: c.sub_subject.clone(),
        },
    ))
}

pub async fn read(r: Reader, prompts: Sender<String>) -> Result<()> {
    println!("launching JetStream Reader");
    let mut messages = r.rx.messages().await?;
    while let Some(Ok(message)) = messages.next().await {
        println!("\n[Q]: {:?}", message.payload.to_owned());
        message.ack().await?;
        // NOTE: maybe we can send an empty string of the conversion fails?
        let prompt = String::from_utf8(message.payload.to_vec())?;
        prompts.send(prompt).await?;
    }

    Ok(())
}

pub async fn write(w: Writer, mut chunks: Receiver<Bytes>) -> Result<()> {
    println!("launching JetStream Writer");
    let mut b = BytesMut::new();
    while let Some(chunk) = chunks.recv().await {
        if chunk.is_empty() {
            let msg = String::from_utf8(b.to_vec()).unwrap();
            println!("\n[A]: {}", msg);
            w.tx.publish(w.subject.to_string(), b.clone().freeze())
                .await?;
            b.clear();
            continue;
        }
        b.extend_from_slice(&chunk);
    }

    Ok(())
}
