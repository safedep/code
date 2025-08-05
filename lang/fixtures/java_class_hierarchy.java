// Java class hierarchy test fixture for testing inheritance resolution
// This file tests various inheritance patterns that should be correctly parsed

import java.util.List;
import java.util.Map;

// Base service class with no inheritance
public abstract class BaseService {
    protected Map<String, Object> config;
    private boolean initialized = false;
    
    public BaseService(Map<String, Object> config) {
        this.config = config;
        this.initialized = true;
    }
    
    public Map<String, Object> getConfig() {
        return config;
    }
    
    public abstract String getServiceType();
    
    public boolean isInitialized() {
        return initialized;
    }
}

// Single inheritance
public class StorageService extends BaseService {
    private String storageType = "generic";
    
    public StorageService(Map<String, Object> config) {
        super(config);
        this.storageType = "generic";
    }
    
    public String uploadFile(String filename) {
        Map<String, Object> config = getConfig(); // Should resolve to BaseService.getConfig
        return "Uploaded " + filename;
    }
    
    @Override
    public String getServiceType() {
        return "storage";
    }
    
    public String getStorageType() {
        return storageType;
    }
}

// Interface for caching functionality
public interface Cacheable {
    void cacheResult(String key, Object value);
    Object getCached(String key);
    void clearCache();
}

// Interface for logging functionality
public interface Loggable {
    void log(String message);
    void setLogLevel(String level);
}

// Multiple interface implementation (Java's version of multiple inheritance)
public class AdvancedStorageService extends StorageService implements Cacheable, Loggable {
    private Map<String, Object> cache;
    private boolean logEnabled = true;
    private String logLevel = "INFO";
    
    public AdvancedStorageService(Map<String, Object> config) {
        super(config);
        this.cache = new HashMap<>();
        this.logEnabled = true;
    }
    
    public String uploadWithCache(String filename) {
        // Call inherited method
        String result = uploadFile(filename);
        // Use cache from Cacheable interface
        cacheResult(filename, result);
        // Use logging from Loggable interface
        log("Cached upload result for " + filename);
        return result;
    }
    
    // Implement Cacheable interface
    @Override
    public void cacheResult(String key, Object value) {
        cache.put(key, value);
    }
    
    @Override
    public Object getCached(String key) {
        return cache.get(key);
    }
    
    @Override
    public void clearCache() {
        cache.clear();
    }
    
    // Implement Loggable interface
    @Override
    public void log(String message) {
        if (logEnabled) {
            System.out.println("[" + logLevel + "] " + message);
        }
    }
    
    @Override
    public void setLogLevel(String level) {
        this.logLevel = level;
    }
}

// Further inheritance from the advanced service
public class CloudStorageService extends AdvancedStorageService {
    private String provider;
    
    public CloudStorageService(Map<String, Object> config, String provider) {
        super(config);
        this.provider = provider;
    }
    
    public String getProvider() {
        return provider;
    }
    
    public void syncToCloud() {
        String serviceType = getServiceType(); // From BaseService through inheritance chain
        log("Syncing to " + provider + ", service type: " + serviceType);
    }
    
    @Override
    public String getServiceType() {
        return "cloud-storage";
    }
}

// Abstract class with annotations
@Service
@Component
public abstract class AbstractProcessor {
    protected String name;
    
    public AbstractProcessor(String name) {
        this.name = name;
    }
    
    @Transactional
    public abstract void process(Object data);
    
    public boolean validate(Object data) {
        return data != null;
    }
    
    public String getName() {
        return name;
    }
}

// Concrete implementation of abstract class
@Repository
public class DataProcessor extends AbstractProcessor {
    private final DatabaseService dbService;
    
    @Autowired
    public DataProcessor(String name, DatabaseService dbService) {
        super(name);
        this.dbService = dbService;
    }
    
    @Override
    @Transactional
    public void process(Object data) {
        if (validate(data)) {
            dbService.save(data);
            System.out.println("Processed " + data + " with " + getName());
        }
    }
}

// Deep inheritance chain (4 levels)
public class Level1 extends BaseService {
    public Level1(Map<String, Object> config) {
        super(config);
    }
    
    public String level1Method() {
        return "level1";
    }
    
    @Override
    public String getServiceType() {
        return "level1";
    }
}

public class Level2 extends Level1 {
    public Level2(Map<String, Object> config) {
        super(config);
    }
    
    public String level2Method() {
        return level1Method() + "_level2";
    }
    
    @Override
    public String getServiceType() {
        return "level2";
    }
}

public class Level3 extends Level2 {
    public Level3(Map<String, Object> config) {
        super(config);
    }
    
    public String level3Method() {
        return level2Method() + "_level3";
    }
}

public class Level4 extends Level3 {
    public Level4(Map<String, Object> config) {
        super(config);
    }
    
    public String level4Method() {
        Map<String, Object> config = getConfig(); // Should resolve through 4 levels to BaseService
        return level3Method() + "_level4";
    }
}

// Generic class with type parameters
public class GenericService<T> extends BaseService {
    private List<T> items;
    
    public GenericService(Map<String, Object> config) {
        super(config);
        this.items = new ArrayList<>();
    }
    
    public void addItem(T item) {
        items.add(item);
    }
    
    public List<T> getItems() {
        return items;
    }
    
    @Override
    public String getServiceType() {
        return "generic";
    }
}

// Inner class testing
public class OuterClass {
    private String outerField = "outer";
    
    public class InnerClass {
        private String innerField = "inner";
        
        public String getOuterField() {
            return outerField; // Access to outer class
        }
    }
    
    public static class StaticInnerClass {
        private String staticField = "static";
        
        public String getStaticField() {
            return staticField;
        }
    }
}

// Interface inheritance
public interface ExtendedInterface extends SimpleInterface {
    void extendedMethod();
    
    @Override
    default String defaultMethod() {
        return "extended_default";
    }
}

// Test instances and method calls for verification
class TestRunner {
    public static void main(String[] args) {
        Map<String, Object> config = new HashMap<>();
        config.put("type", "test");
        
        StorageService storage = new StorageService(config);
        AdvancedStorageService advanced = new AdvancedStorageService(config);
        CloudStorageService cloud = new CloudStorageService(config, "aws");
        Level4 deep = new Level4(config);
        
        // Method calls that should resolve through inheritance
        String storageType = storage.getServiceType();
        String advancedUpload = advanced.uploadWithCache("test.txt");
        cloud.syncToCloud();
        String deepResult = deep.level4Method();
    }
}