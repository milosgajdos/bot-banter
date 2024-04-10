use async_nats::jetstream::{self, consumer::pull, stream::Config};
use futures::StreamExt;
use tokio;

#[tokio::main]
async fn main() -> Result<(), async_nats::Error> {
    let nats_url =
        std::env::var("NATS_URL").unwrap_or_else(|_| "nats://localhost:4222".to_string());

    let client = async_nats::connect(nats_url).await?;

    let js = jetstream::new(client);

    let stream = js
        .get_or_create_stream(Config {
            name: "banter".to_string(),
            ..Default::default()
        })
        .await?;
    println!("connected to stream");

    let cons = stream
        .create_consumer(pull::Config {
            durable_name: Some("rust".to_string()),
            filter_subject: "rust".to_string(),
            ..Default::default()
        })
        .await?;
    println!("created stream consumer");

    let mut messages = cons.messages().await?;
    while let Some(Ok(message)) = messages.next().await {
        println!(
            "Received a JetStream message {:?}",
            message.payload.to_owned()
        );
        message.ack().await?;
        js.publish("go".to_string(), "Thx for the message Golang".into())
            .await?;
    }

    Ok(())
}
