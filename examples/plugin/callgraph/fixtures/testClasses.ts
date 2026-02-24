import { createHash } from 'crypto';
import type { BinaryLike } from 'crypto';

interface Hasher {
  hash(data: string): string;
}

class SHA256Hasher implements Hasher {
  hash(data: string): string {
    return createHash('sha256').update(data).digest('hex');
  }
}

class DoubleHasher extends SHA256Hasher {
  hash(data: string): string {
    const first = super.hash(data);
    return super.hash(first);
  }
}

function processData(hasher: Hasher, data: string): string {
  return hasher.hash(data);
}

const hasher = new DoubleHasher();
const result = processData(hasher, "hello world");
console.log(result);
