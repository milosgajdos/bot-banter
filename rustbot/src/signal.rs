use crate::prelude::*;
use tokio::{self, signal, sync::watch};

pub async fn trap(done: watch::Sender<bool>) -> Result<()> {
    tokio::select! {
        _ = signal::ctrl_c() => {
            println!("shutting down, received SIGINT signal...");
            done.send(true)?;
        }
    }
    Ok(())
}
