class Report
  def emit
    Kernel.system('top-level report')
  end
end

class Guarded
  private :danger

  def danger
    Kernel.sprintf('danger')
  end

  def check
    respond_to?(:danger)
  end
end
