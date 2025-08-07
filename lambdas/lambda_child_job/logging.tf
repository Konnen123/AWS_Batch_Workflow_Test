resource "aws_cloudwatch_log_group" "example" {
  name              = "/aws/lambda/${var.lambda_name}"
  retention_in_days = 1

  tags = {
    Environment = "test"
    Function    = var.lambda_name
  }
}

# CloudWatch Logs policy
resource "aws_iam_policy" "lambda_logging" {
  name        = "lambda_logging_${var.lambda_name}"
  path        = "/"
  description = "IAM policy for logging from Lambda"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents",
        ]
        Resource = ["arn:aws:logs:*:*:*"]
      }
    ]
  })
}

# Attach logging policy to Lambda role
resource "aws_iam_role_policy_attachment" "lambda_logs" {
  role       = aws_iam_role.lambda_role.name
  policy_arn = aws_iam_policy.lambda_logging.arn
}