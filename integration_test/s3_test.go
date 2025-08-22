package integration_test

import (
	"context"
	"github.com/simpplify-org/GO-data-connector-lib/bucket"
	"testing"
)

// ------------------ Test S3 ------------------

func TestS3Integration(t *testing.T) {
	ctx := context.Background()

	s3Client, err := bucket.NewToS3("test", "test", "us-east-1", "teste-bucket", true)

	err = s3Client.CreateBucket(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = s3Client.UploadFile(ctx, "teste.txt", "./teste_local.txt")
	if err != nil {
		t.Fatal(err)
	}

	err = s3Client.DownloadFile(ctx, "teste.txt", "./teste_baixado.txt")
	if err != nil {
		t.Fatal(err)
	}

	err = s3Client.DeleteFile(ctx, "teste.txt")
	if err != nil {
		t.Fatal(err)
	}

	err = s3Client.DeleteBucket(ctx)
	if err != nil {
		t.Fatal(err)
	}
}
