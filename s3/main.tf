resource "aws_s3_bucket" "image_bucket"{
    bucket = var.bucket_name

    tags = {
        Name        = var.bucket_name
        Environment = "Test"
    }
}

output "arn" {
    value = aws_s3_bucket.image_bucket.arn
}
output "name" {
    value = aws_s3_bucket.image_bucket.bucket
}