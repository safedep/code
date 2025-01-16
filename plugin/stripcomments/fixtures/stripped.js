import os from 'os' 
const express = require('express'); 

os.homedir(); 

const x = 1; 


function test() { 
  console.log('test'); 
}
test()

console.log('This is valid code');



class Test {
  constructor() {
    console.log('Test class created');
  }
  
  test(arg1, arg2) {
    console.log(arg1); 
    console.log(arg2); 
  }
}