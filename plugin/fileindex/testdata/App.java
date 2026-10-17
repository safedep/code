package com.example.app;

import com.example.store.Store;
import com.example.util.*;
import static com.example.util.Strings.trim;
import org.springframework.beans.factory.annotation.Autowired;

@RestController
public class App extends Base implements Runnable {
    @Autowired
    private Store store;
    private final List<String> names = new ArrayList<>();

    public App(Store store) {
        super(store);
        this.store = store;
    }

    public static void main(String[] args) {
        App app = new App(new Store());
        app.run();
    }

    @GetMapping("/items")
    public String items() {
        store.save(trim("x"));
        names.forEach(this::log);
        Runnable r = new Runnable() {
            public void run() { log("anon"); }
        };
        start();
        return Helper.name();
    }

    void log(String m) {
        Class.forName(m).getMethod("x").invoke(null);
    }

    abstract void hook();

    static class Inner {
        void work() { new Inner(); }
    }
}

interface Sink extends AutoCloseable {
    void write(String v);

    default void flush() { write(""); }
}

enum Mode {
    ON;

    void apply() { System.out.println(this); }
}
