package com.shop;

import javax.inject.Inject;
import org.springframework.scheduling.annotation.Scheduled;

public class Jobs {
    @Inject
    Repository repository;

    @Scheduled(fixedRate = 1000)
    public void sweep() {
        repository.purge("old");
    }
}
