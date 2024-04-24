use bytes::Bytes;
use clap::Parser;
use prelude::*;
use rodio::{OutputStream, Sink};
use tokio::{
    self, io,
    sync::{mpsc, watch},
};

mod audio;
mod buffer;
mod cli;
mod history;
mod jet;
mod llm;
mod prelude;
mod signal;
mod tts;

#[tokio::main]
async fn main() -> Result<()> {
    let args = cli::App::parse();

    let seed_prompt = args.prompt.seed.unwrap();

    // NOTE: we could also add Stream::builder to the jet module
    // and instead of passing config we could build it by chaining methods.
    let c = jet::Config {
        durable_name: args.bot.name,
        stream_name: args.bot.stream_name,
        pub_subject: args.bot.pub_subject,
        sub_subject: args.bot.sub_subject,
        ..jet::Config::default()
    };
    let s = jet::Stream::new(c).await?;

    // NOTE: we could also add LLM::builder to the llm module
    // and instead of passing config we could build it by chaining methods.
    let c = llm::Config {
        hist_size: args.llm.hist_size,
        model_name: args.llm.model_name,
        seed_prompt: Some(seed_prompt),
        ..llm::Config::default()
    };
    let l = llm::LLM::new(c);

    // NOTE: we could also add TTS::builder to the tts module
    // and instead of passing config we could build it by chaining methods.
    let c = tts::Config {
        voice_id: Some(args.tts.voice_id),
        ..tts::Config::default()
    };
    let t = tts::TTS::new(c);

    let (prompts_tx, prompts_rx) = mpsc::channel::<String>(32);
    let (jet_chunks_tx, jet_chunks_rx) = mpsc::channel::<Bytes>(32);
    let (tts_chunks_tx, tts_chunks_rx) = mpsc::channel::<Bytes>(32);
    let (aud_done_tx, aud_done_rx) = watch::channel(false);

    // NOTE: used for cancellation when SIGINT is trapped.
    let (watch_tx, watch_rx) = watch::channel(false);
    let jet_wr_watch_rx = watch_rx.clone();
    let jet_rd_watch_rx = watch_rx.clone();
    let tts_watch_rx = watch_rx.clone();
    let aud_watch_rx = watch_rx.clone();

    println!("launching workers");

    let (_stream, stream_handle) = OutputStream::try_default().unwrap();
    let sink = Sink::try_new(&stream_handle).unwrap();
    let (audio_wr, audio_rd) = io::duplex(1024);

    let tts_stream = tokio::spawn(t.stream(audio_wr, tts_chunks_rx, tts_watch_rx));
    let llm_stream = tokio::spawn(l.stream(prompts_rx, jet_chunks_tx, tts_chunks_tx, watch_rx));
    let jet_write = tokio::spawn(s.writer.write(jet_chunks_rx, aud_done_rx, jet_wr_watch_rx));
    let jet_read = tokio::spawn(s.reader.read(prompts_tx, jet_rd_watch_rx));
    let audio_task = tokio::spawn(audio::play(audio_rd, sink, aud_done_tx, aud_watch_rx));
    let sig_handler = tokio::spawn(signal::trap(watch_tx));

    // NOTE: we're not waiting for the signal handler here:
    // we abort it once any of the spawn worker tasks exits.
    match tokio::try_join!(tts_stream, llm_stream, jet_write, jet_read, audio_task) {
        Ok(_) => {}
        Err(e) => {
            println!("Error running bot: {}", e);
        }
    }
    sig_handler.abort();
    Ok(())
}
