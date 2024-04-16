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
mod jet;
mod llm;
mod prelude;

#[tokio::main]
async fn main() -> Result<()> {
    let args = cli::App::parse();

    let system_prompt = args.prompt.system.unwrap();
    let seed_prompt = args.prompt.seed.unwrap();
    let prompt = system_prompt + "\n" + &seed_prompt;

    // NOTE: we could also add Stream::builder to jet module
    // and instead of passing config we could build it by chaining methods.
    let c = jet::Config {
        durable_name: args.bot.name,
        stream_name: args.bot.stream_name,
        pub_subject: args.bot.pub_subject,
        sub_subject: args.bot.sub_subject,
        ..jet::Config::default()
    };
    let s = jet::Stream::new(c).await?;

    // NOTE: we could also add LLM::builder to llm module
    // and instead of passing config we could build it by chaining methods.
    let c = llm::Config {
        hist_size: args.llm.hist_size,
        model_name: args.llm.model_name,
        seed_prompt: Some(prompt),
        ..llm::Config::default()
    };
    let l = llm::LLM::new(c);

    let (prompts_tx, prompts_rx) = mpsc::channel::<String>(32);
    let (chunks_tx, chunks_rx) = mpsc::channel::<Bytes>(32);

    // NOTE: used for cancellation when SIGINT is trapped.
    let (watch_tx, watch_rx) = watch::channel(false);
    let jet_wr_watch_rx = watch_rx.clone();
    let jet_rd_watch_rx = watch_rx.clone();

    println!("launching workers");

    let llm_stream = tokio::spawn(l.stream(prompts_rx, chunks_tx, watch_rx));
    let jet_write = tokio::spawn(s.writer.write(chunks_rx, jet_wr_watch_rx));
    let jet_read = tokio::spawn(s.reader.read(prompts_tx, jet_rd_watch_rx));
    let sig_handler: JoinHandle<Result<()>> = tokio::spawn(async move {
        tokio::select! {
            _ = signal::ctrl_c() => {
                println!("shutting down, received SIGINT signal...");
                watch_tx.send(true)?;
            }
        }
        Ok(())
    });

    match tokio::try_join!(llm_stream, jet_write, jet_read) {
        Ok(_) => {}
        Err(e) => {
            println!("Error running bot: {}", e);
        }
    }
    sig_handler.abort();
    Ok(())
}
