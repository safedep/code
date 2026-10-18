require_relative 'lib/helpers'
require_relative 'lib/jobs'

class Runner
  include Jobs::Mixin

  private def secret
    Kernel.system('private-def')
  end

  alias_method :launch, :start_job
  alias boot secret

  def run(name)
    helper_top(name)
    launch
    boot
    puts "#{Jobs::Pretty.new}"
    total = Jobs::Money.new + Jobs::Money.new
    handler = -> { Jobs.lambda_target }
    handler.call
    [1].each(&Jobs.method(:by_method))
    Jobs::Hidden.new.then { Jobs.in_block }
    obj = Jobs::Hidden.new
    obj&.safe_nav
    obj.tap(&:by_symbol)
    Jobs::Deep::Inner.new.reach_up
    Point.new(1).show
  end
end

Point = Struct.new(:x) do
  def show
    Kernel.sleep(1)
  end
end

if __FILE__ == $0
  Runner.new.run(ARGV[0])
end
