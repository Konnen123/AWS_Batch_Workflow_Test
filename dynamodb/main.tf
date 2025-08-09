resource "aws_dynamodb_table" "dynamodb_table_dev" {
  name = var.dynamodb_table_name
  billing_mode   = "PROVISIONED"
  read_capacity  = 2
  write_capacity = 2

  hash_key = "PK"
  range_key = "SK"

  attribute {
    name = "PK"
    type = "S"
  }
  attribute {
    name = "SK"
    type = "S"
  }

  ttl {
    attribute_name = "TTL"
    enabled = true
  }

  tags = {
    Name        = var.dynamodb_table_name
    Environment = "dev"
  }
}

output "arn" {
  value = aws_dynamodb_table.dynamodb_table_dev.arn
}

output "name" {
  value = aws_dynamodb_table.dynamodb_table_dev.name
}