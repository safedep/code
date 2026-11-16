import crypto from 'crypto';

export function digest(text) {
  return crypto.createHash('sha256').update(text).digest('hex');
}
