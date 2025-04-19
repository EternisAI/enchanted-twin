import pandas as pd
import json
import os


def main():
    print("Processing Telegram data...")

    # Ensure output directory exists
    os.makedirs("./output_data", exist_ok=True)

    df_telegram = pd.read_csv("./input_data/telegram.csv")

    # Initialize the chat_id_to_username dictionary
    chat_id_to_username = {}

    # find my username
    my_username = None
    for _, row in df_telegram.iterrows():
        data = json.loads(row["data"])
        if "myMessage" in data:
            my_username = data["from"]
            break

    if my_username is None:
        raise ValueError("My username not found")

    # Populate chat_id_to_username dictionary before processing
    for _, row in df_telegram.iterrows():
        data = json.loads(row["data"])
        if "chatId" in data and data["from"] != my_username:
            chat_id_to_username[data["chatId"]] = data["from"]

    def tf_telegram(row):
        data = json.loads(row["data"])

        if data["type"] == "contact":
            fullName = " ".join([data["firstName"], data["lastName"]])
            if fullName == "":
                return None
            title = "Contact"
            content = f"Contact: {fullName} phone number: {data['phoneNumber']}"
            metadata = {"source": "telegram"}
        elif data["type"] == "message":
            if data["text"] == "":
                return None
            if data["from"] == "":
                return None

            chat_id = data["chatId"]
            conversation_partner_name = chat_id_to_username.get(chat_id, None)
            if conversation_partner_name is None:
                return None

            # Check if myMessage key exists, default to False if it doesn't
            isMyMessage = data["myMessage"]

            title = f"Telegram chat with {conversation_partner_name}"
            if isMyMessage:
                content = (
                    f"Message from me to {conversation_partner_name}: {data['text']}"
                )
            else:
                content = (
                    f"Message from {conversation_partner_name} to me: {data['text']}"
                )

            metadata = {"source": "telegram", "contact": conversation_partner_name}
        else:
            return None

        return {"title": title, "text": content, "metadata": metadata}

    df_telegram["transformed"] = df_telegram.apply(tf_telegram, axis=1)
    df_telegram.rename(columns={"timestamp": "creation_date"}, inplace=True)
    df_telegram.dropna(subset=["transformed"], inplace=True)
    df_telegram = pd.concat(
        [
            df_telegram.drop(["transformed"], axis=1),
            df_telegram["transformed"].apply(pd.Series),
        ],
        axis=1,
    )

    output_path = "./output_data/telegram.csv"
    df_telegram.to_csv(output_path, index=False)
    print(f"Telegram data saved to {output_path}")

    print("Completed Telegram data processing.")


if __name__ == "__main__":
    main()
