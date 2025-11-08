import crypto from "node:crypto";

export function generateSecret(length = 64) {

    const raw = crypto.randomBytes(length);

    // base64url encode (URL-safe, no padding)
    function base64url(buffer: Buffer) {
        return buffer.toString("base64")
            .replace(/\+/g, "-")
            .replace(/\//g, "_")
            .replace(/=+$/, "");
    }

    const secret = base64url(raw);
    return secret;
}


export async function hashPassword(password: string): Promise<string> {
    return await Bun.password.hash(password, {
        algorithm: "argon2id",
        memoryCost: 4,
        timeCost: 3,
    });
}

export async function verifyPassword(password: string, hash: string): Promise<boolean> {
    return await Bun.password.verify(password, hash);
}
