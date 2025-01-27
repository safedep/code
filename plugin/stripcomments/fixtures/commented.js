import os from 'os' // This is a single line comment
const express = require('express'); ////// This is also single line comment

os.homedir(); // This is also a single line comment

const x = 1; /* This is a multi line comment */

/*
Multi line comment docstring
console.log('This is a commented out code');
console.log('This is a commented out code');
*/
// Function with commented-out parts
function sum(a, b) {
// const temp = a + b;
  return a + b; /* This is the actual sum operation */
}
sum(5, 10);

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

switch ("red") {
  case 'blue':
    console.log('Color is blue'); // Comment inside case
    break;
  case 'red':
    /* This is the red color block */
    console.log('Color is red'); // Comment inside case for red
    break;
  default:
    console.log('Unknown color'); // Default case
    // console.log('Color is unknown');
}


