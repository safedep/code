package com.shop.web;

import com.shop.svc.OrderService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class OrderController {
    @Autowired
    private OrderService service;

    @GetMapping("/orders")
    public String list() {
        return service.render("all");
    }

    public String unused() {
        return service.export("all");
    }
}
