# Python class hierarchy test fixture for testing inheritance resolution
# This file tests various inheritance patterns that should be correctly parsed

# Basic class with no inheritance
class BaseService:
    def __init__(self, config):
        self.config = config
        self.initialized = True
    
    def get_config(self):
        return self.config
    
    def status(self):
        return "active"

# Single inheritance
class StorageService(BaseService):
    def __init__(self, config):
        super().__init__(config)
        self.storage_type = "generic"
    
    def upload_file(self, filename):
        config = self.get_config()  # Should resolve to BaseService.get_config
        return f"Uploaded {filename}"
    
    def get_storage_type(self):
        return self.storage_type

# Multiple inheritance with diamond pattern
class Cacheable:
    def __init__(self):
        self.cache = {}
    
    def cache_result(self, key, value):
        self.cache[key] = value
    
    def get_cached(self, key):
        return self.cache.get(key)

class Loggable:
    def __init__(self):
        self.log_enabled = True
    
    def log(self, message):
        if self.log_enabled:
            print(f"LOG: {message}")

# Multiple inheritance combining storage, caching, and logging
class AdvancedStorageService(StorageService, Cacheable, Loggable):
    def __init__(self, config):
        StorageService.__init__(self, config)
        Cacheable.__init__(self)
        Loggable.__init__(self)
        self.advanced_features = True
    
    def upload_with_cache(self, filename):
        # Call inherited method
        result = self.upload_file(filename)
        # Use cache from Cacheable
        self.cache_result(filename, result)
        # Use logging from Loggable
        self.log(f"Cached upload result for {filename}")
        return result

# Inheritance from module-qualified class (simulating external import)
class CloudStorageService(AdvancedStorageService):
    def __init__(self, config, provider):
        super().__init__(config)
        self.provider = provider
    
    def get_provider(self):
        return self.provider
    
    def sync_to_cloud(self):
        status = self.status()  # From BaseService through inheritance chain
        self.log(f"Syncing to {self.provider}, status: {status}")

# Abstract class with decorators
from abc import ABC, abstractmethod

@abstractmethod
class AbstractProcessor(ABC):
    @abstractmethod
    def process(self, data):
        pass
    
    def validate(self, data):
        return data is not None

# Concrete implementation of abstract class
class DataProcessor(AbstractProcessor):
    def __init__(self, name):
        self.name = name
    
    def process(self, data):
        if self.validate(data):
            return f"Processed {data} with {self.name}"
        return "Invalid data"

# Deep inheritance chain
class Level1(BaseService):
    def level1_method(self):
        return "level1"

class Level2(Level1):
    def level2_method(self):
        return self.level1_method() + "_level2"

class Level3(Level2):
    def level3_method(self):
        return self.level2_method() + "_level3"

class Level4(Level3):
    def level4_method(self):
        config = self.get_config()  # Should resolve through 4 levels to BaseService
        return self.level3_method() + "_level4"

# Class with constructor chaining
class ServiceWithDefaults(BaseService):
    def __init__(self, config=None):
        if config is None:
            config = {"default": True}
        super().__init__(config)
        self.has_defaults = True

# Test instances to verify call resolution
base = BaseService({"type": "base"})
storage = StorageService({"type": "storage"})
advanced = AdvancedStorageService({"type": "advanced"})
cloud = CloudStorageService({"type": "cloud"}, "aws")
processor = DataProcessor("main")
deep = Level4({"type": "deep"})

# Method calls that should resolve through inheritance
base_status = base.status()
storage_config = storage.get_config()
advanced_upload = advanced.upload_with_cache("test.txt")
cloud_sync = cloud.sync_to_cloud()
processed_data = processor.process("sample")
deep_result = deep.level4_method()