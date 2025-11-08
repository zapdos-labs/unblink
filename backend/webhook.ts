import type { WebhookMessage } from "~/shared/alert";
import { logger } from "./logger";

export const create_webhook_forward = (props: {
    settings: () => Record<string, string>,
}) => {
    const webhook_forward = async (msg: WebhookMessage) => {
        // Updated
        const webhook_url = props.settings()['alerts.webhook_callback_url'];
        if (!webhook_url) return;

        try {
            await fetch(webhook_url, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify(msg),
            });
        } catch (error) {
            logger.error({ error }, "Error forwarding alert to webhook");
        }
    }

    return webhook_forward;
}