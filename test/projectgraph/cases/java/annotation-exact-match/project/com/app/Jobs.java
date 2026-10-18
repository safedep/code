package com.app;

import java.nio.file.Files;
import java.nio.file.Path;

class Jobs {
    @ScheduledTask
    void cleanup() throws Exception {
        Files.delete(Path.of("cache"));
    }
}
