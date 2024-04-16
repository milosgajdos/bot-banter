use bytes::Bytes;
use clap::Parser;
use prelude::*;
use tokio::{
    self, signal,
    sync::{mpsc, watch},
    task::JoinHandle,
};

mod cli;
mod history;
mod jetstream;
mod llm;
mod prelude;

#[tokio::main]
async fn main() -> Result<()> {
    let args = cli::App::parse();

    let system_prompt = args.prompt.system.unwrap();
    let seed_prompt = args.prompt.seed.unwrap();
    let prompt = system_prompt + "\n" + &seed_prompt;

    let c = jetstream::Config {
        durable_name: args.bot.name,
        stream_name: args.bot.stream_name,
        pub_subject: args.bot.pub_subject,
        sub_subject: args.bot.sub_subject,
        ..jetstream::Config::default()
    };
    let (stream_tx, stream_rx) = jetstream::new(c).await?;

    let (prompts_tx, prompts_rx) = mpsc::channel::<String>(32);
    let (chunks_tx, chunks_rx) = mpsc::channel::<Bytes>(32);

    // NOTE: used for cancellation
    let (watch_tx, watch_rx) = watch::channel(false);
    let jetwr_watch_rx = watch_rx.clone();
    let jetrd_watch_rx = watch_rx.clone();

    println!("launching workers");

    let c = llm::Config {
        hist_size: args.llm.hist_size,
        model_name: args.llm.model_name,
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
