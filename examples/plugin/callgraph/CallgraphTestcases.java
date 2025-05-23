import java.util.stream.Collectors;
import com.sun.activation.registries.MailcapFile;

import java.util.*;
import java.util.function.*;

public class CallgraphTestcases {
    public static void main(String[] args) {
        System.out.println(Collectors.toList());
        MailcapFile mailcapFile = new MailcapFile();
        System.out.println(mailcapFile.getMimeTypes());
        
        // === Inheritance and overriding ===
        Base baseRef = new Derived();
        baseRef.sayHello(); // Should invoke Derived.sayHello

        // === Method overloading ===
        Derived derived = new Derived();
        derived.overloaded("Overload String");
        derived.overloaded(123);

        // === Static and Instance methods ===
        CallgraphTestcases test = new CallgraphTestcases();
        test.start();

        // === Method reference ===
        Runnable r = test::printMessage;
        r.run();

        // === Lambda expression ===
        Function<Integer, Integer> square = (x) -> x * x;
        System.out.println("Square(4): " + square.apply(4));

        // === Lambda callback ===
        test.executeWithCallback(() -> System.out.println("Running callback"));

        // === Recursive call ===
        Recursive.recursiveCall(3);

        // === Inner class ===
        Inner inner = test.new Inner();
        inner.callOuter();

        // === Static nested class ===
        StaticNested nested = new StaticNested();
        nested.staticNestedCall();

        // === Local class inside method ===
        Helper helper = new Helper();
        helper.helperMethod();

        // === Interface implementation ===
        Action action = new ActionImpl();
        action.perform();

        // === Collection traversal with method reference ===
        List<String> items = Arrays.asList("x", "y", "z");
        items.forEach(test::printItem);
    }

    public void start() {
        System.out.println("== Starting Test ==");
        printMessage();

        try {
            riskyMethod();
        } catch (Exception e) {
            System.out.println("Caught: " + e.getMessage());
        } finally {
            cleanup();
        }
    }

    public void printMessage() {
        System.out.println("Hello from printMessage");
    }

    public void riskyMethod() throws Exception {
        throw new Exception("Simulated Exception");
    }

    public void cleanup() {
        System.out.println("Cleanup complete");
    }

    public void printItem(String item) {
        System.out.println("Item: " + item);
    }

    public void executeWithCallback(Runnable callback) {
        System.out.println("Before callback");
        callback.run();
        System.out.println("After callback");
    }

    // === Non-static inner class ===
    public class Inner {
        public void callOuter() {
            System.out.println("Calling from Inner");
            printMessage();
        }
    }

    // === Static nested class ===
    public static class StaticNested {
        public void staticNestedCall() {
            System.out.println("Inside StaticNested class");
        }
    }
}

// === Inheritance and Overriding ===
abstract class Base {
    public abstract void sayHello();
}

class Derived extends Base {
    @Override
    public void sayHello() {
        System.out.println("Hello from Derived");
    }

    public void overloaded(String msg) {
        System.out.println("Overloaded with String: " + msg);
    }

    public void overloaded(int num) {
        System.out.println("Overloaded with int: " + num);
    }
}

// === Local class inside a method ===
class Helper {
    public void helperMethod() {
        System.out.println("Inside Helper");

        class Local {
            void call() {
                System.out.println("Inside local class method");
            }
        }

        Local local = new Local();
        local.call();
    }
}

// === Recursive call ===
class Recursive {
    public static void recursiveCall(int n) {
        if (n == 0) {
            System.out.println("Reached base case");
            return;
        }
        System.out.println("Recursing with n = " + n);
        recursiveCall(n - 1);
    }
}

// === Interface and Implementation ===
interface Action {
    void perform();
}

class ActionImpl implements Action {
    public void perform() {
        System.out.println("Action performed!");
    }
}
