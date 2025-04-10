from pprint import pprint
from xyzprintmodule import xyzprint, xyzprint2, xyzprint3, xyzprint4 as xprint

def fn1():
  xprint("very outer fn1")
fn1()

def fn2():
  def fn1():
    xyzprint("fn1 inside fn2")
  fn1()

  def fn3():
    def fn4():
      def fn1():
        xyzprint3("fn1 inside fn4 inside fn3")
      xyzprint2("fn4 inside fn3")
      fn1() # must call fn1 inside fn4
    fn1() # must call fn1 inside fn2
    fn4()
  fn3()

fn2()
  