// Simple Java classes for basic testing
// Tests fundamental class resolution without complex inheritance

public class SimpleClass {
    private int value = 42;
    
    public SimpleClass() {
        this.value = 42;
    }
    
    public int getValue() {
        return value;
    }
}

class ClassWithMethods {
    private String name;
    
    public ClassWithMethods(String name) {
        this.name = name;
    }
    
    public String getName() {
        return name;
    }
    
    public void setName(String newName) {
        this.name = newName;
    }
    
    public String process() {
        return "Processing " + name;
    }
}

public class ClassWithFields {
    public static final String CLASS_VAR = "shared";
    private String instanceVar = "unique";
    protected int counter = 0;
    
    public void increment() {
        counter++;
    }
    
    public int getCounter() {
        return counter;
    }
}

// Class with annotations
@Component
public class AnnotatedClass {
    private String value;
    
    @Autowired
    public AnnotatedClass(String value) {
        this.value = value;
    }
    
    @Override
    public String toString() {
        return value;
    }
}

// Interface for baseline testing
public interface SimpleInterface {
    void simpleMethod();
    
    default String defaultMethod() {
        return "default";
    }
}

// Class with no inheritance for baseline testing
final class StandaloneClass {
    public void standaloneMethod() {
        System.out.println("standalone");
    }
}