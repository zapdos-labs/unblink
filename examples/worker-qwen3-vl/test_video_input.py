#!/usr/bin/env python3
"""
Test script to validate that video input produces <|video_pad|> tokens
"""
import torch
from PIL import Image, ImageDraw, ImageFont
from transformers import Qwen2VLForConditionalGeneration, AutoProcessor
from qwen_vl_utils import process_vision_info


def create_test_frames(num_frames=5, width=448, height=448):
    """Create synthetic frames with frame numbers"""
    frames = []
    colors = [(255, 0, 0), (0, 255, 0), (0, 0, 255), (255, 255, 0), (255, 0, 255)]

    for i in range(num_frames):
        img = Image.new('RGB', (width, height), colors[i % len(colors)])
        draw = ImageDraw.Draw(img)
        # Draw frame number
        draw.text((width//2, height//2), f"Frame {i}", fill=(255, 255, 255))
        frames.append(img)

    return frames


def test_video_input(frames, processor):
    """Test Flow 1: Video input (should produce <|video_pad|>)"""
    print("\n" + "="*80)
    print("TEST 1: Video Input (Flow 1)")
    print("="*80)

    messages = [{
        "role": "user",
        "content": [
            {
                "type": "video",
                "video": frames  # Pass list of PIL images
            },
            {
                "type": "text",
                "text": "Describe this video."
            }
        ]
    }]

    # Process vision info
    image_inputs, video_inputs = process_vision_info(messages)

    # Generate text with template
    text = processor.apply_chat_template(messages, tokenize=False, add_generation_prompt=True)

    # Tokenize
    inputs = processor(
        text=[text],
        images=image_inputs,
        videos=video_inputs,
        padding=True,
        return_tensors="pt"
    )

    # Decode to see tokens
    tokens_decoded = processor.tokenizer.decode(inputs['input_ids'][0])

    print(f"\nVideo inputs: {len(video_inputs) if video_inputs else 0} video(s)")
    if video_inputs:
        print(f"  Video shape: {video_inputs[0].shape}")  # Should be (T, C, H, W)
    print(f"Image inputs: {len(image_inputs) if image_inputs else 0} image(s)")

    # Print the PRE-tokenization text
    print(f"\nPRE-tokenization text (from apply_chat_template):")
    print("-" * 80)
    # Show just the vision part
    if '<|vision_start|>' in text:
        vision_start = text.find('<|vision_start|>')
        vision_end = text.find('Describe this video.')
        vision_snippet = text[max(0, vision_start-50):vision_end]
        print(vision_snippet + "...")
    print("-" * 80)
    print("NOTE: Timestamps are injected DURING processor() call, not visible here.")

    # Print the full decoded text to inspect tokens
    print(f"\nFull decoded text:")
    print("-" * 80)
    print(tokens_decoded)
    print("-" * 80)

    # Check for video_pad tokens
    has_video_pad = '<|video_pad|>' in tokens_decoded
    has_image_pad = '<|image_pad|>' in tokens_decoded

    print(f"\nToken Analysis:")
    print(f"  Contains <|video_pad|>: {has_video_pad}")
    print(f"  Contains <|image_pad|>: {has_image_pad}")

    # Count video_pad tokens
    if has_video_pad:
        pad_count = tokens_decoded.count('<|video_pad|>')
        print(f"  Video pad token count: {pad_count}")

    # Check grid dimensions
    if 'video_grid_thw' in inputs:
        print(f"\nGrid dimensions (video_grid_thw): {inputs['video_grid_thw']}")
        # Should be [T, H, W] with T > 1

    return has_video_pad, inputs


def test_manual_image_input(frames, processor):
    """Test Flow 2: Manual image input (produces <|image_pad|>)"""
    print("\n" + "="*80)
    print("TEST 2: Manual Image Input (Flow 2)")
    print("="*80)

    # Construct message with individual images
    content = []
    for i, frame in enumerate(frames):
        content.append({
            "type": "text",
            "text": f"<{i * 5.0} seconds>"  # Manual timestamps
        })
        content.append({
            "type": "image",
            "image": frame
        })
    content.append({
        "type": "text",
        "text": "Describe this sequence."
    })

    messages = [{
        "role": "user",
        "content": content
    }]

    # Process vision info
    image_inputs, video_inputs = process_vision_info(messages)

    # Generate text with template
    text = processor.apply_chat_template(messages, tokenize=False, add_generation_prompt=True)

    # Tokenize
    inputs = processor(
        text=[text],
        images=image_inputs,
        videos=video_inputs,
        padding=True,
        return_tensors="pt"
    )

    # Decode to see tokens
    tokens_decoded = processor.tokenizer.decode(inputs['input_ids'][0])

    print(f"\nVideo inputs: {len(video_inputs) if video_inputs else 0} video(s)")
    print(f"Image inputs: {len(image_inputs) if image_inputs else 0} image(s)")

    # Print the PRE-tokenization text (shows manual timestamps)
    print(f"\nPRE-tokenization text (with manual timestamps in content):")
    print("-" * 80)
    # Show just the first two vision blocks to see manual timestamps
    if '<|vision_start|>' in text:
        vision_start = text.find('<|vision_start|>')
        second_vision = text.find('<|vision_start|>', vision_start + 1)
        third_vision = text.find('<|vision_start|>', second_vision + 1)
        vision_snippet = text[max(0, vision_start-20):third_vision] if third_vision != -1 else text[max(0, vision_start-20):second_vision+100]
        print(vision_snippet + "...")
    print("-" * 80)

    # Print the full decoded text to inspect tokens
    print(f"\nFull decoded text:")
    print("-" * 80)
    print(tokens_decoded)
    print("-" * 80)

    # Check for video_pad tokens
    has_video_pad = '<|video_pad|>' in tokens_decoded
    has_image_pad = '<|image_pad|>' in tokens_decoded

    print(f"\nToken Analysis:")
    print(f"  Contains <|video_pad|>: {has_video_pad}")
    print(f"  Contains <|image_pad|>: {has_image_pad}")

    # Count image_pad tokens
    if has_image_pad:
        pad_count = tokens_decoded.count('<|image_pad|>')
        print(f"  Image pad token count: {pad_count}")

    # Check grid dimensions
    if 'image_grid_thw' in inputs:
        print(f"\nGrid dimensions (image_grid_thw): {inputs['image_grid_thw'][:3]}")
        # Should be [[1, H, W], [1, H, W], ...] with T=1 for each

    return has_image_pad, inputs


def main():
    print("Loading Qwen3-VL model...")

    # Use small model for testing
    model_name = "Qwen/Qwen2-VL-2B-Instruct"

    processor = AutoProcessor.from_pretrained(model_name)

    # Create test frames
    print("\nCreating 5 test frames...")
    frames = create_test_frames(num_frames=5)

    # Test 1: Video input
    has_video_pad, video_inputs = test_video_input(frames, processor)

    # Test 2: Manual image input
    has_image_pad, image_inputs = test_manual_image_input(frames, processor)

    # Summary
    print("\n" + "="*80)
    print("SUMMARY")
    print("="*80)
    print(f"✓ Flow 1 (video type) produces <|video_pad|>: {has_video_pad}")
    print(f"✓ Flow 2 (manual images) produces <|image_pad|>: {has_image_pad}")

    if has_video_pad and has_image_pad:
        print("\n✅ SUCCESS: Both approaches work as expected!")
        print("   - Use Flow 1 (video type) to get temporal reasoning with <|video_pad|>")
        print("   - Flow 2 produces <|image_pad|> (independent images, no temporal context)")
    else:
        print("\n❌ UNEXPECTED: Token types don't match expected behavior")
        if not has_video_pad:
            print("   - Video input did NOT produce <|video_pad|>")
        if not has_image_pad:
            print("   - Manual image input did NOT produce <|image_pad|>")


if __name__ == "__main__":
    main()
