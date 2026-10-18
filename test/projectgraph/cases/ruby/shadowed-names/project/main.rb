require_relative 'lib/top'
require_relative 'lib/billing/report'
require_relative 'lib/billing/runner'
require_relative 'lib/shapes'
require_relative 'lib/tools'

def upload_all
  upload = 1
  upload
end

# Kernel.fork is in a comment
if __FILE__ == $0
  Billing::Runner.new.go
  A::Foo.new.run
  Foo.run
  Tools.new.valid?
  upload_all
  Guarded.new.check
  puts 'Kernel.fork in a string'
end
