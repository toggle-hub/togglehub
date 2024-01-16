import { Stack, StackProps } from "aws-cdk-lib";
import * as lambda from "aws-cdk-lib/aws-lambda";
import * as sqs from "aws-cdk-lib/aws-sqs";
import * as eventSources from "aws-cdk-lib/aws-lambda-event-sources";
import { Construct } from "constructs";

export class InfraStack extends Stack {
  constructor(scope: Construct, id: string, props?: StackProps) {
    super(scope, id, props);

    const emailQueue = new sqs.Queue(this, "EmailVerificationQueue", {
      queueName: "EmailVerificationQueue",
    });

    const emailVerificationHandler = new lambda.Function(
      this,
      "EmailVerificationHandler",
      {
        runtime: lambda.Runtime.NODEJS_20_X,
        code: lambda.Code.fromAsset("dist"),
        functionName: "EmailVerificationHandler",
        handler: "emailVerification.handler",
      }
    );

    const eventSource = new eventSources.SqsEventSource(emailQueue);

    emailVerificationHandler.addEventSource(eventSource);
  }
}
