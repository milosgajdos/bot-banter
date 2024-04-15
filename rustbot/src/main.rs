use bytes::Bytes;
use clap::Parser;
use prelude::*;
use tokio::{
    self, signal,
    sync::{mpsc, watch},
    task::JoinHandle,
};

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
    #[arg(long, default_value_t = 50)]
    #[arg(short = 't')]
    hist_size: usize,
    #[arg(short, long, default_value = DEFAULT_MODEL_NAME)]
    model_name: String,
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

    // NOTE: used for cancellation
    let (watch_tx, watch_rx) = watch::channel(false);
    let jetwr_watch_rx = watch_rx.clone();
    let jetrd_watch_rx = watch_rx.clone();

    println!("launching {} workers", BOT_NAME);

    let c = llm::Config {
        hist_size: args.hist_size,
        model_name: args.model_name,
        seed_prompt: Some(prompt),
        ..llm::Config::default()
    };
    let llm_stream = tokio::spawn(llm::stream(prompts_rx, chunks_tx, watch_rx, c));
    let jet_write = tokio::spawn(jetstream::write(stream_tx, chunks_rx, jetwr_watch_rx));
    let jet_read = tokio::spawn(jetstream::read(stream_rx, prompts_tx, jetrd_watch_rx));
    let sig_handler: JoinHandle<Result<()>> = tokio::spawn(async move {
        tokio::select! {
            _ = signal::ctrl_c() => {
                println!("shutting down, received SIGINT signal...");
                watch_tx.send(true)?;
            }
        }
        Ok(())
    });

    let _ = tokio::try_join!(llm_stream, jet_write, jet_read)?;
    sig_handler.abort();
    Ok(())
}
