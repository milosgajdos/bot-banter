use bytes::{Bytes, BytesMut};
use clap::Parser;
use prelude::*;
use rodio::{Decoder, OutputStream, Sink};
use std::io::Cursor;
use tokio::{
    self,
    io::{self, AsyncReadExt},
    signal,
    sync::{mpsc, watch},
    task::JoinHandle,
};

mod buffer;
mod cli;
mod history;
mod jet;
mod llm;
mod prelude;
mod tts;

#[tokio::main]
async fn main() -> Result<()> {
    let args = cli::App::parse();

    let system_prompt = args.prompt.system.unwrap();
    let seed_prompt = args.prompt.seed.unwrap();
    let prompt = system_prompt + "\n" + &seed_prompt;

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
        seed_prompt: Some(prompt),
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

    // NOTE: used for cancellation when SIGINT is trapped.
    let (watch_tx, watch_rx) = watch::channel(false);
    let jet_wr_watch_rx = watch_rx.clone();
    let jet_rd_watch_rx = watch_rx.clone();
    let tts_watch_rx = watch_rx.clone();

    println!("launching workers");

    let (_stream, stream_handle) = OutputStream::try_default().unwrap();
    let sink = Sink::try_new(&stream_handle).unwrap();
    let (mut audio_wr, mut audio_rd) = io::duplex(1024);

    let tts_stream =
        tokio::spawn(async move { t.stream(&mut audio_wr, tts_chunks_rx, tts_watch_rx).await });
    let llm_stream = tokio::spawn(l.stream(prompts_rx, jet_chunks_tx, tts_chunks_tx, watch_rx));
    let jet_write = tokio::spawn(s.writer.write(jet_chunks_rx, jet_wr_watch_rx));
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
    let play_task = tokio::spawn(async move {
        let mut audio_data = BytesMut::new();
        while let Ok(chunk) = audio_rd.read_buf(&mut audio_data).await {
            if chunk == 0 {
                break;
            }
            if audio_data.len() > AUDIO_BUFFER_SIZE {
                let cursor = Cursor::new(audio_data.clone().freeze().to_vec());
                match Decoder::new(cursor) {
                    Ok(source) => {
                        sink.append(source);
                        audio_data.clear(); // Clear the buffer on successful append
                    }
                    Err(e) => {
                        eprintln!("Failed to decode received audio: {}", e);
                    }
                }
            }
        }

        // Flush any remaining data at the end
        if !audio_data.is_empty() {
            let cursor = Cursor::new(audio_data.to_vec());
            match Decoder::new(cursor) {
                Ok(source) => sink.append(source),
                Err(e) => println!("Remaining data could not be decoded: {}", e),
            }
        }
        sink.sleep_until_end();
    });

    match tokio::try_join!(tts_stream, llm_stream, jet_write, jet_read, play_task) {
        Ok(_) => {}
        Err(e) => {
            println!("Error running bot: {}", e);
        }
    }
    sig_handler.abort();
    Ok(())
}
