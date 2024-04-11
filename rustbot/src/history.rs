use std::fmt;

struct History {
    data: Vec<String>,
    size: usize,
    pos: usize,
}

impl History {
    fn new(size: usize) -> Self {
        History {
            data: vec![String::new(); size],
            size,
            pos: 0,
        }
    }

    fn add(&mut self, element: String) {
        self.data[self.pos] = element;
        self.pos = (self.pos + 1) % self.size;
    }

    fn chunks(&self) -> String {
        let mut result = Vec::with_capacity(self.size);
        for i in 0..self.size {
            result.push(&self.data[(self.pos + i) % self.size][..]);
        }
        result.join("\n")
    }
}

impl fmt::Display for History {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        let mut result = Vec::with_capacity(self.size);
        for i in 0..self.size {
            result.push(&self.data[(self.pos + i) % self.size][..]);
        }
        write!(f, "{}", result.join("\n"))
    }
}
