use bytes::Bytes;
use prelude::*;
use tokio::{self, signal, sync::mpsc};

mod history;
mod jetstream;
mod llm;
mod prelude;

#[tokio::main]
async fn main() -> Result<()> {
    let c = jetstream::Config::default();
    let (stream_tx, stream_rx) = jetstream::new(c).await?;

    let (prompts_tx, prompts_rx) = mpsc::channel::<String>(32);
    let (chunks_tx, chunks_rx) = mpsc::channel::<Bytes>(32);

    println!("launching {} workers", BOT_NAME);

    let c = llm::Config::default();
    let llm_stream = tokio::spawn(llm::stream(prompts_rx, chunks_tx, c));
    let jet_write = tokio::spawn(jetstream::write(stream_tx, chunks_rx));
    let jet_read = tokio::spawn(jetstream::read(stream_rx, prompts_tx));

    tokio::pin!(llm_stream, jet_write, jet_read);

    tokio::select! {
        result = &mut llm_stream => {
            if let Err(e) = result {
                eprintln!("shutting down, llm_stream encountered an error: {}", e);
                jet_write.abort();
                jet_read.abort();
                return Err(e.into());
            }
        }
        result = &mut jet_write => {
            if let Err(e) = result {
                eprintln!("shutting down, jetstream_write encountered an error: {}", e);
                llm_stream.abort();
                jet_read.abort();
                return Err(e.into());
            }
        }
        result = &mut jet_read => {
            if let Err(e) = result {
                eprintln!("shutting down, jetstream_read encountered an error: {}", e);
                llm_stream.abort();
                jet_write.abort();
                return Err(e.into());
            }
        }
        _ = signal::ctrl_c() => {
            println!("shutting down, received SIGINT signal...");
            llm_stream.abort();
            jet_write.abort();
            jet_read.abort();
            return Ok(());
        }
    }

    Ok(())
}
