package com.shop.util;

import org.apache.commons.text.StringEscapeUtils;

public class Text {
    public static String render(Object v) {
        return StringEscapeUtils.escapeHtml4(String.valueOf(v));
    }
}
