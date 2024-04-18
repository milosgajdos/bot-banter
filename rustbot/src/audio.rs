use crate::prelude::*;
use bytes::BytesMut;
use rodio::{Decoder, Sink};
use std::io::Cursor;
use tokio::{
    self,
    io::{self, AsyncReadExt},
};

pub async fn play(mut audio_rd: io::DuplexStream, sink: Sink) -> Result<()> {
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
    Ok(())
}
