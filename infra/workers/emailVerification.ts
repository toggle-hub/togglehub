import { SQSEvent } from "aws-lambda";

export const handler = async (event: SQSEvent) => {
  const res = await fetch("https://api.resend.com/emails", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer RESEND_API_KEY`,
    },
    body: JSON.stringify({
      from: "Toggle Labs <onboarding@togglelabs.net>",
      to: [""],
      subject: "hello world",
      html: "<strong>Onboarding</strong>",
    }),
  });

  if (!res.ok) {
    // send to queue/logger
  }

  return;
};
