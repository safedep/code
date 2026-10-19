pub struct Thing<A> {
    items: Vec<A>,
}

impl<A> Thing<A> {
    pub fn new() -> Self {
        Thing { items: Vec::new() }
    }

    pub fn from_vec(items: Vec<A>) -> Self {
        Thing { items }
    }

    pub fn insert(&mut self, item: A) {
        self.items.push(item);
    }

    pub fn child(&self) -> &Thing<A> {
        self
    }
}

impl<A: Clone> Clone for Thing<A> {
    fn clone(&self) -> Self {
        Thing { items: self.items.clone() }
    }
}

pub fn new_thing() -> Thing<u8> {
    Thing::new()
}

pub fn try_thing() -> Option<Thing<u8>> {
    Some(Thing::new())
}

pub fn load_thing() -> Result<Thing<u8>, String> {
    Ok(Thing::new())
}

pub trait Doer {
    fn run(&self);
}

impl<A> Doer for Thing<A> {
    fn run(&self) {}
}

pub fn doer() -> Box<dyn Doer> {
    Box::new(Thing::<u8>::new())
}

pub trait Runner {
    fn start(&self);
}
