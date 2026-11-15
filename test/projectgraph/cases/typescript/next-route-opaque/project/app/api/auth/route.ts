import NextAuth from 'next-auth';
import { a1 } from 'plug';

export default NextAuth({ callbacks: { async signIn() { a1(); } } });
