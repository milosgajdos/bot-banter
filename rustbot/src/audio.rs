use crate::prelude::*;
use bytes::BytesMut;
use rodio::{Decoder, Sink};
use std::io::Cursor;
use tokio::{
    self,
    io::{self, AsyncReadExt},
    sync::watch,
    time::{self, Duration, Instant},
};

pub async fn play(
    mut audio_rd: io::DuplexStream,
    sink: Sink,
    audio_done: watch::Sender<bool>,
    mut done: watch::Receiver<bool>,
) -> Result<()> {
    println!("launching audio player");
    let mut audio_data = BytesMut::new();
    // TODO: make this a cli switch as this value has been picked rather arbitrarily
    let interval_duration = Duration::from_millis(AUDIO_INTERVAL);
    let mut interval = time::interval(interval_duration);
    let mut last_play_time = Instant::now();
    let mut has_played_audio = false;

    loop {
        tokio::select! {
            _ = done.changed() => {
                if *done.borrow() {
                    break;
                }
            }
            result = audio_rd.read_buf(&mut audio_data) => {
                if let Ok(chunk) = result {
                    if chunk == 0 {
                        break;
                    }
                    if audio_data.len() > AUDIO_BUFFER_SIZE {
                        // NOTE: this avoids unnecessary data duplication and manages the buffer efficiently
                        let cursor = Cursor::new(audio_data.split_to(AUDIO_BUFFER_SIZE).freeze().to_vec());
                        match Decoder::new(cursor) {
                            Ok(source) => {
                                sink.append(source);
                                last_play_time = Instant::now();
                                has_played_audio = true;
                            }
                            Err(e) => {
                                eprintln!("Failed to decode received audio: {}", e);
                            }
                        }
                    }
                }
            }
            _ = interval.tick() => {
                // No audio data received in the past interval_duration ms and we've
                // already played some audio -- that means we can proceed with dialogue
                // by writing a followup question into JetStream through jet::writer.
                if has_played_audio && last_play_time.elapsed() >= interval_duration && sink.empty() {
                    if !audio_data.is_empty() {
                        let cursor = Cursor::new(audio_data.clone().freeze().to_vec());
                        if let Ok(source) = Decoder::new(cursor) {
                            sink.append(source);
                            audio_data.clear();
                        }
                    }
                    sink.sleep_until_end();
                    // NOTE: notify jet::writer
                    audio_done.send(true)?;
                    has_played_audio = false;
                }
            }
        }
    }

    // Flush any remaining data
    if !audio_data.is_empty() {
        let cursor = Cursor::new(audio_data.clone().to_vec());
        if let Ok(source) = Decoder::new(cursor) {
            sink.append(source);
        }
    }
    if !sink.empty() {
        sink.sleep_until_end();
    }
    Ok(())
}
