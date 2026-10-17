package com.app;

import org.springframework.boot.CommandLineRunner;
import org.springframework.stereotype.Component;

@Component
public class Startup implements CommandLineRunner {
    @Override
    public void run(String... args) {
        Runtime.getRuntime().exec("migrate");
    }
}
