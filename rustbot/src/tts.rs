use crate::{buffer, prelude::*};
use bytes::Bytes;
use playht_rs::api::{self, stream::TTSStreamReq, tts::Quality};
use tokio::{self, sync::mpsc::Receiver, sync::watch};

#[derive(Debug, Clone)]
pub struct Config {
    pub voice_id: Option<String>,
    pub quality: Option<Quality>,
    pub speed: Option<f32>,
    pub sample_rate: Option<i32>,
    pub buf_size: usize,
}

impl Default for Config {
    fn default() -> Self {
        Config {
            voice_id: Some(DEFAULT_VOICE_ID.to_string()),
            quality: Some(Quality::Low),
            speed: Some(1.0),
            sample_rate: Some(24000),
            buf_size: MAX_TTS_BUFFER_SIZE,
        }
    }
}

pub struct TTS {
    client: api::Client,
    config: Config,
}

impl TTS {
    pub fn new(c: Config) -> TTS {
        TTS {
            client: api::Client::new(),
            config: c,
        }
    }

    pub async fn stream<W>(
        self,
        mut w: W,
        mut chunks: Receiver<Bytes>,
        mut done: watch::Receiver<bool>,
    ) -> Result<()>
    where
        W: tokio::io::AsyncWriteExt + Unpin,
    {
        println!("launching TTS stream");
        let mut buf = buffer::Buffer::new(self.config.buf_size);
        let mut req = TTSStreamReq {
            voice: self.config.voice_id,
            quality: self.config.quality,
            speed: self.config.speed,
            sample_rate: self.config.sample_rate,
            ..Default::default()
        };

        loop {
            tokio::select! {
                _ = done.changed() => {
                    if *done.borrow() {
                        return Ok(())
                    }
                },
                Some(chunk) = chunks.recv() => {
                    if chunk.is_empty() {
                        let text = String::from_utf8(buf.as_bytes().to_vec())?;
                        req.text = Some(text);
                        self.client.write_audio_stream(&mut w, &req).await?;
                        buf.reset();
                        continue
                    }
                    match buf.write(chunk.as_ref()) {
                        Ok(_) => {},
                        Err(e) => {
                            let text = String::from_utf8(buf.as_bytes().to_vec())?;
                            req.text = Some(text);
                            self.client.write_audio_stream(&mut w, &req).await?;
                            buf.reset();
                            let rem = chunk.len() - e.bytes_written;
                            let chunk_slice = chunk.as_ref();
                            buf.write(&chunk_slice[rem..])?;
                        }
                    }
                }
            }
        }
    }
}
