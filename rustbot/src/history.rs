use std::collections::VecDeque;
use std::fmt;

#[derive(Clone, Debug)]
pub struct History {
    data: VecDeque<String>,
    size: usize,
}

impl History {
    pub fn new(size: usize) -> Self {
        History {
            data: VecDeque::with_capacity(size),
            size,
        }
    }

    pub fn add(&mut self, element: String) {
        if self.data.len() == self.size {
            self.data.pop_front();
        }
        self.data.push_back(element);
    }

    pub fn string(&self) -> String {
        let mut s = String::new();
        for string in &self.data {
            s.push_str(string);
            s.push('\n');
        }
        s.pop(); // Remove the last newline character
        s
    }
}

impl fmt::Display for History {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        let mut result = Vec::with_capacity(self.size);
        for string in &self.data {
            result.push(string.as_str());
        }
        write!(f, "{}", result.join("\n"))
    }
}
