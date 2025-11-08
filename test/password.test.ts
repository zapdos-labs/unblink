import { expect, test } from "bun:test";
import { hashPassword, verifyPassword } from "~/backend/auth";

test("password hashing", async () => {
    const password = "my_secure_password";
    const hash = await hashPassword(password);
    expect(hash).not.toBe(password);
    const isValid = await verifyPassword(password, hash);
    expect(isValid).toBe(true);
});
