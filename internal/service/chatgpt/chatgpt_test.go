package chatgpt

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestOpenAITypedObjectWrapper_UnmarshalJSON(t *testing.T) {
	payload := `
{
  "id": "foo",
  "status": "completed",
  "output": [
    {
      "type": "message",
      "id": "msg1",
      "status": "completed",
 	  "role": "assistant",
      "content": [
		{
          "type": "output_text",
          "text": "This is a response"
		}
      ]
    },
    {
      "type": "custom_tool_call",
      "call_id": "call_id_1",
      "id": "tool_call_1",
	  "status": "completed",
      "name": "MyTool",
      "input": "Tool input"
    },
    {
      "type": "unknown_type",
      "field1": "Field 1",
      "field2": "Field 2"
    }
  ]
}`
	var resp ResponsesResponse
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		t.Error(err)
	}

	if resp.ID != "foo" {
		t.Errorf("resp.ID != \"foo\", got: %s", resp.ID)
	}
	if resp.Status != "completed" {
		t.Errorf("resp.Status != \"completed\", got: %s", resp.Status)
	}
	output0, ok := resp.Output[0].Obj.(*Message)
	if !ok {
		t.Errorf("resp.Output[0].Obj is not of type Message, got: %T", resp.Output[0].Obj)
	}
	if output0.ID != "msg1" {
		t.Errorf("output0.ID != \"msg1\", got: %s", output0.ID)
	}

	content0, ok := output0.Content[0].Obj.(*OutputText)
	if !ok {
		t.Errorf("output0.Content[0] is not of type OutputText, got: %T", output0.Content[0])
	}
	if content0.Text != "This is a response" {
		t.Errorf("content0.Text != \"This is a response\", got: %s", content0.Text)
	}

	output1, ok := resp.Output[1].Obj.(*CustomToolCall)
	if !ok {
		t.Errorf("resp.Output[1].Obj is not of type CustomToolCall, got: %T", resp.Output[1].Obj)
	}
	if output1.CallID != "call_id_1" {
		t.Errorf("output1.CallID != \"call_id_1\", got: %s", output1.CallID)
	}

	output2, ok := resp.Output[2].Obj.(*RawObject)
	if !ok {
		t.Errorf("resp.Output[2].Obj is not of type RawObject, got: %T", resp.Output[2].Obj)
	}
	if output2.GetType() != "unknown_type" {
		t.Errorf("output2.GetTypeName() != \"unknown_type\", got: %s", output2.GetType())
	}
}

func TestOpenAITypedObjectWrapper_MarshalJSON(t *testing.T) {
	obj := OpenAIObjectWrapper{
		Obj: &Message{
			Content: []OpenAIObjectWrapper{
				{
					Obj: &OutputText{
						Text: "foo",
					},
				},
				{
					Obj: &InputText{
						Text: "bar",
					},
				},
			},
		},
	}
	payload, err := json.Marshal(&obj)
	if err != nil {
		t.Error(err)
	}

	fmt.Printf("payload: %v\n", string(payload))
	rawMap := make(map[string]interface{})
	if err := json.Unmarshal(payload, &rawMap); err != nil {
		t.Error(err)
	}

	if rawMap["type"] != "message" {
		t.Errorf(".type != \"message\", got: %s", rawMap["type"])
	}
	contents := rawMap["content"].([]any)
	content0 := contents[0].(map[string]any)
	if content0["type"] != "output_text" {
		t.Errorf(".content.[0].type != \"output_text\", got: %s", content0["type"])
	}
	if content0["text"] != "foo" {
		t.Errorf(".content.[0].text != \"foo\", got: %s", content0["text"])
	}

	content1 := contents[1].(map[string]any)
	if content1["type"] != "input_text" {
		t.Errorf(".content.[1].type != \"input_text\", got: %s", content1["type"])
	}
	if content1["text"] != "bar" {
		t.Errorf(".content.[1].text != \"bar\", got: %s", content1["text"])
	}
}
