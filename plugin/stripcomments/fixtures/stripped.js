import os from 'os' 
const express = require('express'); 

os.homedir(); 

const x = 1; 



function sum(a, b) {

  return a + b; 
}
sum(5, 10);


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

switch ("red") {
  case 'blue':
    console.log('Color is blue'); 
    break;
  case 'red':
    
    console.log('Color is red'); 
    break;
  default:
    console.log('Unknown color'); 
    
}