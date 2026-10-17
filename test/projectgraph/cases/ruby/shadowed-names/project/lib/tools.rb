class Tools
  def valid?
    true
  end

  def valid
    Kernel.syscall(1)
  end

  def upload
    Kernel.srand(1)
  end
end
