pub type Result<T> = std::result::Result<T, Box<dyn std::error::Error + Send + Sync>>;

pub const HISTORY_SIZE: usize = 50;
pub const MODEL_NAME: &str = "llama2:latest";
pub const NATS_DEFAULT_URL: &str = "nats://localhost:4222";
pub const STREAM_NAME: &str = "banter";
pub const BOT_NAME: &str = "rustbot";
pub const BOT_SUB_SUBJECT: &str = "rust";
pub const BOT_PUB_SUBJECT: &str = "go";
