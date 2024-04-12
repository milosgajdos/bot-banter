use std::fmt;

pub struct History {
    data: Vec<String>,
    size: usize,
    pos: usize,
}

impl History {
    pub fn new(size: usize) -> Self {
        History {
            data: vec![String::new(); size],
            size,
            pos: 0,
        }
    }

    pub fn add(&mut self, element: String) {
        self.data[self.pos] = element;
        self.pos = (self.pos + 1) % self.size;
    }

    pub fn string(&self) -> String {
        let mut s = String::with_capacity(
            self.data
                .iter()
                .take(self.size)
                .map(|s| s.len())
                .sum::<usize>()
                + self.size,
        );
        for i in 0..self.size {
            s.push_str(&self.data[(self.pos + i) % self.size]);
            s.push('\n');
        }
        s.pop();
        s
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
