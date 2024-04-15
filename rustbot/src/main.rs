use bytes::Bytes;
use clap::Parser;
use prelude::*;
use tokio::{self, signal, sync::mpsc};

mod history;
mod jetstream;
mod llm;
mod prelude;

#[derive(Parser, Debug)]
#[command(version, about, long_about = None)]
struct Args {
    #[arg(long, default_value = DEFAULT_SYSTEM_PROMPT)]
    system_prompt: Option<String>,
    #[arg(long, default_value = DEFAULT_SEED_PROMPT)]
    seed_prompt: Option<String>,
    #[arg(long)]
    boot_prompt: Option<String>,
}

#[tokio::main]
async fn main() -> Result<()> {
    let args = Args::parse();

    let system_prompt = args.system_prompt.unwrap();
    let seed_prompt = args.seed_prompt.unwrap();
    let prompt = system_prompt + "\n" + &seed_prompt;

    let c = jetstream::Config::default();
    let (stream_tx, stream_rx) = jetstream::new(c).await?;

    let (prompts_tx, prompts_rx) = mpsc::channel::<String>(32);
    let (chunks_tx, chunks_rx) = mpsc::channel::<Bytes>(32);

    println!("launching {} workers", BOT_NAME);

    let c = llm::Config {
        seed_prompt: Some(prompt),
        ..llm::Config::default()
    };
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
