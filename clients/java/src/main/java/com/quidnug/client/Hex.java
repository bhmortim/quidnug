package com.quidnug.client;

/** Tiny hex codec so we don't pull in Apache Commons for this alone. */
final class Hex {
    private static final char[] HEX = "0123456789abcdef".toCharArray();

    private Hex() {}

    static String encode(byte[] b) {
        char[] out = new char[b.length * 2];
        for (int i = 0; i < b.length; i++) {
            out[i * 2]     = HEX[(b[i] >> 4) & 0x0f];
            out[i * 2 + 1] = HEX[b[i]        & 0x0f];
        }
        return new String(out);
    }

    static byte[] decode(String s) {
        if ((s.length() & 1) != 0) throw new IllegalArgumentException("odd hex length");
        byte[] out = new byte[s.length() / 2];
        for (int i = 0; i < out.length; i++) {
            int hi = Character.digit(s.charAt(i * 2),     16);
            int lo = Character.digit(s.charAt(i * 2 + 1), 16);
            if (hi < 0 || lo < 0) throw new IllegalArgumentException("bad hex char");
            out[i] = (byte) ((hi << 4) | lo);
        }
        return out;
    }
}
