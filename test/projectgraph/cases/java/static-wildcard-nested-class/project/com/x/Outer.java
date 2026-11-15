package com.x;

import com.y.*;
import static com.x.Outer.Inner.*;

public class Outer extends Base {
    public static class Inner {
        public static void help() {}
    }

    void run() {
        baseMethod();
        help();
    }
}
