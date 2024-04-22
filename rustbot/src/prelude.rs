pub type Result<T> = std::result::Result<T, Box<dyn std::error::Error + Send + Sync>>;

pub const HISTORY_SIZE: usize = 50;
pub const DEFAULT_MODEL_NAME: &str = "llama2:latest";
pub const NATS_DEFAULT_URL: &str = "nats://localhost:4222";
pub const STREAM_NAME: &str = "banter";
pub const BOT_NAME: &str = "rustbot";
pub const BOT_SUB_SUBJECT: &str = "rust";
pub const BOT_PUB_SUBJECT: &str = "go";

pub const DEFAULT_SEED_PROMPT: &str = "You are a Rust programming language expert \
    and a helpful AI assistant trying to learn about Go programming language. \
    You will answer questions ONLY about Rust and ONLY ask questions about Go. \
    You do NOT explain how Go works. You are NOT Go expert! You ONLY compare Go \
    to Rust. When you receive the response you will evaluate it from a Rust programmer \
    point of view and ask followup questions about Go. NEVER use emojis in your answers! \
    Your answers must NOT be longer than 100 words! \
    Question: What is the biggest strength of Rust?
Assistant: Rust's biggest strength lies in its focus on safety, particularly memory \
safety, without sacrificing performance. Can you tell me what are some of the biggest \
strengths of Go that make it stand out from other programming languages?
Question: ";

pub const DEFAULT_VOICE_ID: &str =
    "s3://voice-cloning-zero-shot/b3def996-302e-486f-a234-172fa0279f0e/anthonysaad/manifest.json";
pub const MAX_TTS_BUFFER_SIZE: usize = 1000;
pub const AUDIO_BUFFER_SIZE: usize = 1024 * 10;
pub const AUDIO_INTERVAL: u64 = 200;
