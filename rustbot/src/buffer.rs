use bytes::{BufMut, Bytes, BytesMut};
use std::error::Error;
use std::fmt;

#[derive(Debug)]
pub struct BufferFullError {
    pub bytes_written: usize,
}

impl fmt::Display for BufferFullError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(f, "buffer is full, {} bytes written", self.bytes_written)
    }
}

impl Error for BufferFullError {}

pub struct Buffer {
    buffer: BytesMut,
    max_size: usize,
}

impl Buffer {
    pub fn new(max_size: usize) -> Self {
        Buffer {
            buffer: BytesMut::with_capacity(max_size),
            max_size,
        }
    }

    pub fn write(&mut self, data: &[u8]) -> Result<usize, BufferFullError> {
        let available = self.max_size - self.buffer.len();
        let write_len = std::cmp::min(data.len(), available);

        self.buffer.put_slice(&data[..write_len]);

        if self.buffer.len() == self.max_size {
            return Err(BufferFullError {
                bytes_written: write_len,
            });
        }
        Ok(write_len)
    }

    pub fn reset(&mut self) {
        self.buffer.clear();
    }

    pub fn as_bytes(&self) -> Bytes {
        self.buffer.clone().freeze()
    }
}
