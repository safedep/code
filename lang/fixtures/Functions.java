package lang.fixtures;

import java.util.List;

public class MyClassWithFunctions {

    private String field;

    // Constructor
    public MyClassWithFunctions(String field) {
        this.field = field;
    }

    // Public method
    public String publicMethod(int i) {
        return "public";
    }

    // Protected method
    protected void protectedMethod() {
    }

    // Private method
    private boolean privateMethod(String s) {
        return s.isEmpty();
    }

    // Static method
    public static void staticMethod() {
    }

    // Method with annotation
    @Override
    public String toString() {
        return field;
    }
}

// A simple function (static method in a container class in Java)
class TestFunctions {
    public static int add(int a, int b) {
        return a + b;
    }
}
