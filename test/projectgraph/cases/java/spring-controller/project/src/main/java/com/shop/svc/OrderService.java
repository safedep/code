package com.shop.svc;

import freemarker.template.Template;

public class OrderService {
    public String render(String name) {
        return new Template(name).toString();
    }

    public String export(String name) {
        return java.util.Base64.getEncoder().encodeToString(name.getBytes());
    }
}
