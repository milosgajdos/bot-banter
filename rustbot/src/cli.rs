use crate::prelude::*;
use clap::{Args, Parser};

#[derive(Parser, Debug)]
#[command(version, about, long_about = None)]
pub struct App {
    #[command(flatten)]
    pub prompt: Prompt,
    #[command(flatten)]
    pub llm: LLM,
    #[command(flatten)]
    pub bot: Bot,
    #[command(flatten)]
    pub tts: TTS,
}

#[derive(Args, Debug)]
pub struct Prompt {
    #[arg(long, default_value = DEFAULT_SEED_PROMPT, help = "instruction prompt")]
    pub seed: Option<String>,
}

#[derive(Args, Debug)]
pub struct LLM {
    #[arg(long, default_value_t = 50)]
    #[arg(short = 't', help = "chat history size")]
    pub hist_size: usize,
    #[arg(short, long, default_value = DEFAULT_MODEL_NAME, help = "LLM model")]
    pub model_name: String,
}

#[derive(Args, Debug)]
pub struct Bot {
    #[arg(short, long = "bot-name", default_value = BOT_NAME, help = "bot name")]
    pub name: String,
    #[arg(short, long, default_value = STREAM_NAME, help = "jetstram name")]
    pub stream_name: String,
    #[arg(short, long, default_value = BOT_PUB_SUBJECT, help = "jetstream publish subject")]
    pub pub_subject: String,
    #[arg(short = 'b', long, default_value = BOT_SUB_SUBJECT, help = "jetstream subscribe subject")]
    pub sub_subject: String,
}

#[derive(Args, Debug)]
pub struct TTS {
    #[arg(short, default_value = DEFAULT_VOICE_ID, help = "bot name")]
    pub voice_id: String,
}
