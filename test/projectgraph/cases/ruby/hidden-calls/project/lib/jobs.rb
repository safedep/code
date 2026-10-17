module Jobs
  module Mixin
    def start_job
      Process.spawn('mixin')
    end
  end

  class Pretty
    def to_s
      Open3.capture2('to_s')
    end
  end

  class Money
    def +(other)
      Kernel.exec('plus')
    end
  end

  def self.lambda_target
    Kernel.fork
  end

  def self.by_method(x)
    Kernel.spawn('by_method')
  end

  def self.in_block
    Kernel.syscall(1)
  end

  class Hidden
    def safe_nav
      Kernel.trap('safe')
    end

    def by_symbol
      Kernel.exit!('symbol')
    end
  end

  class Helper
    def self.assist
      Kernel.abort('assist')
    end
  end

  module Deep
    class Inner
      def reach_up
        Helper.assist
      end
    end
  end
end
