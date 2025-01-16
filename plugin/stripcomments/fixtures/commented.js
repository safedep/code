import os from 'os' // This is a single line comment
const express = require('express'); ////// This is also single line comment

os.homedir(); // This is also a single line comment

const x = 1; /* This is a multi line comment */

/*
Multi line comment docstring
console.log('This is a commented out code');
console.log('This is a commented out code');
*/
function test() { 
  console.log('test'); // This is a single line comment
}
test()
// console.log('This is a commented out code');
console.log('This is valid code');
// console.log('This is a commented out code');

/**
 * This is a docstring for Class Test
 * This is a docstring for Class Test
 * This is a docstring for Class Test
 */
class Test {
  constructor() {
    console.log('Test class created');
  }
  /**
   * arg1 - argument 1 of type string
   * arg2 - argument 2 of type string
   */
  test(arg1, arg2) {
    console.log(arg1); /**/
    console.log(arg2); /// 
  }
}


