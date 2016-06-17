package formatter

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/docker/engine-api/types"
)

func TestContainerContextWrite(t *testing.T) {
	unixTime := time.Now().AddDate(0, 0, -1).Unix()
	expectedTime := time.Unix(unixTime, 0).String()

	contexts := []struct {
		context  ContainerContext
		expected string
	}{
		// Errors
		{
			ContainerContext{
				Context: Context{
					Format: "{{InvalidFunction}}",
				},
			},
			`Template parsing error: template: :1: function "InvalidFunction" not defined
`,
		},
		{
			ContainerContext{
				Context: Context{
					Format: "{{nil}}",
				},
			},
			`Template parsing error: template: :1:2: executing "" at <nil>: nil is not a command
`,
		},
		// Table Format
		{
			ContainerContext{
				Context: Context{
					Format: "table",
				},
			},
			`CONTAINER ID        IMAGE               COMMAND             CREATED             STATUS              PORTS               NAMES
containerID1        ubuntu              ""                  24 hours ago                                                foobar_baz
containerID2        ubuntu              ""                  24 hours ago                                                foobar_bar
`,
		},
		{
			ContainerContext{
				Context: Context{
					Format: "table {{.Image}}",
				},
			},
			"IMAGE\nubuntu\nubuntu\n",
		},
		{
			ContainerContext{
				Context: Context{
					Format: "table {{.Image}}",
				},
				Size: true,
			},
			"IMAGE\nubuntu\nubuntu\n",
		},
		{
			ContainerContext{
				Context: Context{
					Format: "table {{.Image}}",
					Quiet:  true,
				},
			},
			"IMAGE\nubuntu\nubuntu\n",
		},
		{
			ContainerContext{
				Context: Context{
					Format: "table",
					Quiet:  true,
				},
			},
			"containerID1\ncontainerID2\n",
		},
		// Raw Format
		{
			ContainerContext{
				Context: Context{
					Format: "raw",
				},
			},
			fmt.Sprintf(`container_id: containerID1
image: ubuntu
command: ""
created_at: %s
status: 
names: foobar_baz
labels: 
ports: 

container_id: containerID2
image: ubuntu
command: ""
created_at: %s
status: 
names: foobar_bar
labels: 
ports: 

`, expectedTime, expectedTime),
		},
		{
			ContainerContext{
				Context: Context{
					Format: "raw",
				},
				Size: true,
			},
			fmt.Sprintf(`container_id: containerID1
image: ubuntu
command: ""
created_at: %s
status: 
names: foobar_baz
labels: 
ports: 
size: 0 B

container_id: containerID2
image: ubuntu
command: ""
created_at: %s
status: 
names: foobar_bar
labels: 
ports: 
size: 0 B

`, expectedTime, expectedTime),
		},
		{
			ContainerContext{
				Context: Context{
					Format: "raw",
					Quiet:  true,
				},
			},
			"container_id: containerID1\ncontainer_id: containerID2\n",
		},
		// Custom Format
		{
			ContainerContext{
				Context: Context{
					Format: "{{.Image}}",
				},
			},
			"ubuntu\nubuntu\n",
		},
		{
			ContainerContext{
				Context: Context{
					Format: "{{.Image}}",
				},
				Size: true,
			},
			"ubuntu\nubuntu\n",
		},
	}

	for _, context := range contexts {
		containers := []types.Container{
			{ID: "containerID1", Names: []string{"/foobar_baz"}, Image: "ubuntu", Created: unixTime},
			{ID: "containerID2", Names: []string{"/foobar_bar"}, Image: "ubuntu", Created: unixTime},
		}
		out := bytes.NewBufferString("")
		context.context.Output = out
		context.context.Containers = containers
		context.context.Write()
		actual := out.String()
		if actual != context.expected {
			t.Fatalf("Expected \n%s, got \n%s", context.expected, actual)
		}
		// Clean buffer
		out.Reset()
	}
}

func TestContainerContextWriteWithNoContainers(t *testing.T) {
	out := bytes.NewBufferString("")
	containers := []types.Container{}

	contexts := []struct {
		context  ContainerContext
		expected string
	}{
		{
			ContainerContext{
				Context: Context{
					Format: "{{.Image}}",
					Output: out,
				},
			},
			"",
		},
		{
			ContainerContext{
				Context: Context{
					Format: "table {{.Image}}",
					Output: out,
				},
			},
			"IMAGE\n",
		},
		{
			ContainerContext{
				Context: Context{
					Format: "{{.Image}}",
					Output: out,
				},
				Size: true,
			},
			"",
		},
		{
			ContainerContext{
				Context: Context{
					Format: "table {{.Image}}",
					Output: out,
				},
				Size: true,
			},
			"IMAGE\n",
		},
		{
			ContainerContext{
				Context: Context{
					Format: "table {{.Image}}\t{{.Size}}",
					Output: out,
				},
			},
			"IMAGE               SIZE\n",
		},
		{
			ContainerContext{
				Context: Context{
					Format: "table {{.Image}}\t{{.Size}}",
					Output: out,
				},
				Size: true,
			},
			"IMAGE               SIZE\n",
		},
	}

	for _, context := range contexts {
		context.context.Containers = containers
		context.context.Write()
		actual := out.String()
		if actual != context.expected {
			t.Fatalf("Expected \n%s, got \n%s", context.expected, actual)
		}
		// Clean buffer
		out.Reset()
	}
}

func TestImageContextWrite(t *testing.T) {
	unixTime := time.Now().AddDate(0, 0, -1).Unix()
	expectedTime := time.Unix(unixTime, 0).String()

	contexts := []struct {
		context  ImageContext
		expected string
	}{
		// Errors
		{
			ImageContext{
				Context: Context{
					Format: "{{InvalidFunction}}",
				},
			},
			`Template parsing error: template: :1: function "InvalidFunction" not defined
`,
		},
		{
			ImageContext{
				Context: Context{
					Format: "{{nil}}",
				},
			},
			`Template parsing error: template: :1:2: executing "" at <nil>: nil is not a command
`,
		},
		// Table Format
		{
			ImageContext{
				Context: Context{
					Format: "table",
				},
			},
			`REPOSITORY          TAG                 IMAGE ID            CREATED             SIZE
image               tag1                imageID1            24 hours ago        0 B
image               tag2                imageID2            24 hours ago        0 B
<none>              <none>              imageID3            24 hours ago        0 B
`,
		},
		{
			ImageContext{
				Context: Context{
					Format: "table {{.Repository}}",
				},
			},
			"REPOSITORY\nimage\nimage\n<none>\n",
		},
		{
			ImageContext{
				Context: Context{
					Format: "table {{.Repository}}",
				},
				Digest: true,
			},
			`REPOSITORY          DIGEST
image               sha256:cbbf2f9a99b47fc460d422812b6a5adff7dfee951d8fa2e4a98caa0382cfbdbf
image               <none>
<none>              <none>
`,
		},
		{
			ImageContext{
				Context: Context{
					Format: "table {{.Repository}}",
					Quiet:  true,
				},
			},
			"REPOSITORY\nimage\nimage\n<none>\n",
		},
		{
			ImageContext{
				Context: Context{
					Format: "table",
					Quiet:  true,
				},
			},
			"imageID1\nimageID2\nimageID3\n",
		},
		{
			ImageContext{
				Context: Context{
					Format: "table",
					Quiet:  false,
				},
				Digest: true,
			},
			`REPOSITORY          TAG                 DIGEST                                                                    IMAGE ID            CREATED             SIZE
image               tag1                sha256:cbbf2f9a99b47fc460d422812b6a5adff7dfee951d8fa2e4a98caa0382cfbdbf   imageID1            24 hours ago        0 B
image               tag2                <none>                                                                    imageID2            24 hours ago        0 B
<none>              <none>              <none>                                                                    imageID3            24 hours ago        0 B
`,
		},
		{
			ImageContext{
				Context: Context{
					Format: "table",
					Quiet:  true,
				},
				Digest: true,
			},
			"imageID1\nimageID2\nimageID3\n",
		},
		// Raw Format
		{
			ImageContext{
				Context: Context{
					Format: "raw",
				},
			},
			fmt.Sprintf(`repository: image
tag: tag1
image_id: imageID1
created_at: %s
virtual_size: 0 B

repository: image
tag: tag2
image_id: imageID2
created_at: %s
virtual_size: 0 B

repository: <none>
tag: <none>
image_id: imageID3
created_at: %s
virtual_size: 0 B

`, expectedTime, expectedTime, expectedTime),
		},
		{
			ImageContext{
				Context: Context{
					Format: "raw",
				},
				Digest: true,
			},
			fmt.Sprintf(`repository: image
tag: tag1
digest: sha256:cbbf2f9a99b47fc460d422812b6a5adff7dfee951d8fa2e4a98caa0382cfbdbf
image_id: imageID1
created_at: %s
virtual_size: 0 B

repository: image
tag: tag2
digest: <none>
image_id: imageID2
created_at: %s
virtual_size: 0 B

repository: <none>
tag: <none>
digest: <none>
image_id: imageID3
created_at: %s
virtual_size: 0 B

`, expectedTime, expectedTime, expectedTime),
		},
		{
			ImageContext{
				Context: Context{
					Format: "raw",
					Quiet:  true,
				},
			},
			`image_id: imageID1
image_id: imageID2
image_id: imageID3
`,
		},
		// Custom Format
		{
			ImageContext{
				Context: Context{
					Format: "{{.Repository}}",
				},
			},
			"image\nimage\n<none>\n",
		},
		{
			ImageContext{
				Context: Context{
					Format: "{{.Repository}}",
				},
				Digest: true,
			},
			"image\nimage\n<none>\n",
		},
	}

	for _, context := range contexts {
		images := []types.Image{
			{ID: "imageID1", RepoTags: []string{"image:tag1"}, RepoDigests: []string{"image@sha256:cbbf2f9a99b47fc460d422812b6a5adff7dfee951d8fa2e4a98caa0382cfbdbf"}, Created: unixTime},
			{ID: "imageID2", RepoTags: []string{"image:tag2"}, Created: unixTime},
			{ID: "imageID3", RepoTags: []string{"<none>:<none>"}, RepoDigests: []string{"<none>@<none>"}, Created: unixTime},
		}
		out := bytes.NewBufferString("")
		context.context.Output = out
		context.context.Images = images
		context.context.Write()
		actual := out.String()
		if actual != context.expected {
			t.Fatalf("Expected \n%s, got \n%s", context.expected, actual)
		}
		// Clean buffer
		out.Reset()
	}
}

func TestImageContextWriteWithNoImage(t *testing.T) {
	out := bytes.NewBufferString("")
	images := []types.Image{}

	contexts := []struct {
		context  ImageContext
		expected string
	}{
		{
			ImageContext{
				Context: Context{
					Format: "{{.Repository}}",
					Output: out,
				},
			},
			"",
		},
		{
			ImageContext{
				Context: Context{
					Format: "table {{.Repository}}",
					Output: out,
				},
			},
			"REPOSITORY\n",
		},
		{
			ImageContext{
				Context: Context{
					Format: "{{.Repository}}",
					Output: out,
				},
				Digest: true,
			},
			"",
		},
		{
			ImageContext{
				Context: Context{
					Format: "table {{.Repository}}",
					Output: out,
				},
				Digest: true,
			},
			"REPOSITORY          DIGEST\n",
		},
	}

	for _, context := range contexts {
		context.context.Images = images
		context.context.Write()
		actual := out.String()
		if actual != context.expected {
			t.Fatalf("Expected \n%s, got \n%s", context.expected, actual)
		}
		// Clean buffer
		out.Reset()
	}
}
