# AWS_Lambda_Zip_Workflow_Test
A flow that is responsible to zip multiple images from S3 using Lambdas. 

A lambda is responsible to create child jobs that zip images in parallel (fan-out pattern) and finally a reducer that adds all the zip files into one (fan-in pattern).
