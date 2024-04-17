use crate::{history, prelude::*};
use bytes::Bytes;
use ollama_rs::{generation::completion::request::GenerationRequest, Ollama};
use tokio::{
    self,
    sync::mpsc::{Receiver, Sender},
    sync::watch,
    task::JoinHandle,
};
use tokio_stream::StreamExt;

#[derive(Clone, Debug)]
pub struct Config {
    pub hist_size: usize,
    pub model_name: String,
    pub seed_prompt: Option<String>,
}

impl Default for Config {
    fn default() -> Self {
        Config {
            hist_size: HISTORY_SIZE,
            model_name: DEFAULT_MODEL_NAME.to_string(),
            seed_prompt: None,
        }
    }
}

pub struct LLM {
    client: Ollama,
    model_name: String,
    hist_size: usize,
    seed_prompt: Option<String>,
}

impl LLM {
    pub fn new(c: Config) -> Self {
        let ollama = Ollama::default();
        LLM {
            client: ollama,
            model_name: c.model_name,
            hist_size: c.hist_size,
            seed_prompt: c.seed_prompt,
        }
    }

    pub async fn stream(
        self,
        mut prompts: Receiver<String>,
        jet_chunks: Sender<Bytes>,
        tts_chunks: Sender<Bytes>,
        mut done: watch::Receiver<bool>,
    ) -> Result<()> {
        println!("launching LLM stream");
        use history::History;
        let mut history = History::new(self.hist_size);

        if let Some(seed_prompt) = self.seed_prompt {
            println!("Seed prompt: {}", seed_prompt);
            history.add(seed_prompt.to_string());
        }

        loop {
            tokio::select! {
                _ = done.changed() => {
                    if *done.borrow() {
                        return Ok(())
                    }
                },
                Some(prompt) = prompts.recv() => {
                    history.add(prompt.clone());
                    let mut stream = self.client
                        .generate_stream(GenerationRequest::new(
                            self.model_name.clone(),
                            history.string(),
                        ))
                        .await?;

                    while let Some(res) = stream.next().await {
                        let responses = res?;
                        for resp in responses {
                            let resp_bytes = Bytes::from(resp.response);
                            let jet_bytes = resp_bytes.clone();
                            let jet_ch = jet_chunks.clone();
                            let jet_task: JoinHandle<Result<()>> = tokio::spawn(async move {
                               jet_ch.send(Bytes::from(jet_bytes)).await?;
                               Ok(())
                            });
                            let tts_bytes = resp_bytes.clone();
                            let tts_ch = tts_chunks.clone();
                            let tts_task: JoinHandle<Result<()>> = tokio::spawn(async move {
                                tts_ch.send(Bytes::from(tts_bytes)).await?;
                                Ok(())
                            });
                            match tokio::try_join!(jet_task, tts_task) {
                                Ok(_) => {}
                                Err(e) => {
                                    return Err(Box::new(e));
                                }
                            }
                        }
                    }
                },
            }
        }
    }
}
