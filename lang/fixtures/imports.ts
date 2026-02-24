// === ES Module imports (shared with JS) ===

// Default import
import express from 'express';
import DotEnv from 'dotenv';

// Wildcard/namespace import
import * as lodash from 'lodash';
import * as mathUtils from './math-utils';

// Named imports
import { EADDRINUSE, EACCES, EAGAIN } from 'constants';
import { hex } from 'chalk/ansi-styles';

// Aliased named imports
import { useEffect, useState as useMyState } from 'react';

// Mixed default and named imports
import ReactDOM, { render, flushSync as flushIt } from 'react-dom';

// Dynamic import
const dynamicModule = await import('./dynamic-module.js');

// From file (relative)
import config from './config';
import helper from '../utils/helper';

// Scoped packages
import { Component } from '@angular/core';

// === TypeScript-specific imports ===

// Type-only import (default)
import type Express from 'express';

// Type-only named imports
import type { Request, Response } from 'express';

// Type-only import with alias
import type { Config as AppConfig } from './config';

// === CommonJS require (also valid in TS) ===
const buffer = require('buffer');
const { patch } = require('virtual-dom');
const { bar, foo: fooAlias } = require('@xyz/pqr');
