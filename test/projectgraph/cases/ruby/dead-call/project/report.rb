require 'open3'

class Report
  def self.print(name)
    puts name
  end

  def self.upload(name)
    Open3.capture2('scp', name)
  end
end
