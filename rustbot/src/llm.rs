use crate::{history, prelude::*};
use bytes::Bytes;
use ollama_rs::{generation::completion::request::GenerationRequest, Ollama};
use tokio::{
    self,
    sync::mpsc::{Receiver, Sender},
};
use tokio_stream::StreamExt;

#[derive(Clone, Debug)]
pub struct Config {
    pub hist_size: usize,
    pub model_name: String,
}

impl Default for Config {
    fn default() -> Self {
        Config {
            hist_size: HISTORY_SIZE,
            model_name: MODEL_NAME.to_string(),
        }
    }
}

pub async fn stream(mut prompts: Receiver<String>, chunks: Sender<Bytes>, c: Config) -> Result<()> {
    println!("launching LLM stream");
    use history::History;
    let ollama = Ollama::default();
    let mut history = History::new(c.hist_size);

    while let Some(prompt) = prompts.recv().await {
        history.add(prompt.clone());
        let mut stream = ollama
            .generate_stream(GenerationRequest::new(
                c.model_name.clone(),
                history.string(),
            ))
            .await?;

        while let Some(res) = stream.next().await {
            let responses = res?;
            for resp in responses {
                chunks.send(Bytes::from(resp.response)).await?;
            }
        }
    }

    Ok(())
}
