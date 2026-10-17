require_relative 'lib/report'
require_relative 'lib/report_upload'

if __FILE__ == $0
  Billing::Report.new.publish
end
