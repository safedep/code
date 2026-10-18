package com.y;

import org.springframework.stereotype.Component;

@Component
class Jobs {
    @org.springframework.scheduling.annotation.Scheduled(fixedRate = 1000)
    void tick() {
        java.nio.file.Files.delete(java.nio.file.Path.of("tmp"));
    }
}
