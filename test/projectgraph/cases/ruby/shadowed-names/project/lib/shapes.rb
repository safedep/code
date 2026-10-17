module A
  class Foo
    def run
      Kernel.spawn('a foo')
    end
  end
end

module B
  class Foo
    def run
      Kernel.fork
    end
  end
end

class Foo
  def self.run
    Kernel.abort('class method')
  end

  def run
    Kernel.trap('instance method')
  end
end
