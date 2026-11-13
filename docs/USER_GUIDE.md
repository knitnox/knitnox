# User guide

Knitnox is a private AI harness for one person or a small team. It is one program with one portable database, and it works with a local model or a cloud provider.

With others, it puts your people and one AI in the same chat: you ask together, everyone sees the same answer, and it keeps what you agreed. On your own, it is a private assistant with a memory, a to-do list and helpers that work while you are away. It works the same for a small team, a club, a group of friends or a family.

This guide has two parts. The first part starts from what you need and is all most people read. The rest is a reference: every section stands on its own, so jump to the one you want.

> **In a hurry?** Read "Start in three steps" and "What do you need?".

## What do you need?

| The problem | What to use | Read |
|---|---|---|
| Everyone asked a different AI and now there are five answers | A **group**: one question, one answer, in front of everybody | [Settle a question together](#1-settle-a-question-together) |
| "Didn't we change that last week?" Nobody remembers what was agreed | The **group's memory**: what was decided and whose message it came from | [Keep track of what was decided](#2-keep-track-of-what-was-decided) |
| Different people ask you for different things and you lose track | The **Plan Book**: your tasks, each with who added it | [Plan Book](#plan-book-your-tasks-and-projects) |
| You are busy and miss what happened | The **Feed**: short updates from your chats and your agents | [Stay up to date without looking everywhere](#3-stay-up-to-date-without-looking-everywhere) |
| You repeat the same facts about yourself in every chat | **Memory**: the AI remembers what you tell it | [Memory](#memory-what-the-ai-remembers-about-you) |
| Something should happen every morning without you | An **agent**: a helper with a job and a schedule | [Agents](#agents-workers-that-run-in-the-background) |
| The AI should do things, not only talk: search the web, read a calendar, use a service of yours | A **tool server (MCP)**: gives the AI tools. Add one to your chats, to a group or to one agent | [Give the AI tools](#give-the-ai-tools-mcp-servers) |
| You did not follow something in a group and do not want to ask in front of everyone | **Your own chat, connected to the group**: ask there in private | [Ask in private about a group](#ask-in-private-about-a-group) |
| You want to show someone a chat, but not let them write | A **read-only link** | [Show a chat to others](#show-a-chat-to-others-a-read-only-link) |

## How the parts fit together

Knitnox has two sides and one bridge between them.

| Side | Part | What it is | What it connects to |
|---|---|---|---|
| Shared with your people | **Group** | A chat for several people. Everyone asks, one AI answers | Has its own memory |
| | **Group memory** | What the group decided, and whose message it came from. Only the AI writes it | Can be read from your own chat |
| The bridge | **Take to my chat** | The arrow under a message in a group. Opens a chat of your own with that message quoted. Nobody in the group sees what you write there | Reads the group, never writes to it |
| Yours alone | **Your own chat** | You and the AI. It can use your private things | Plan Book, Feed, Memory |
| | **Plan Book** | Your tasks, each with who added it | Filled from your chats; agents can read it |
| | **Feed** | Short updates in one place | Filled by your chats and your agents |
| | **Memory** | What the AI knows about you | Used in your own chats, never in a group |
| | **Agents** | Helpers with a job. They work by themselves on a schedule, day and night, or only when you start them | Post to your Feed, can read your Plan Book |
| Everywhere | **Tools (MCP servers)** | What lets the AI act outside Knitnox: web search, a calendar, a service of your own | Your own chats use yours, a group has its own, an agent can have its own |

In a group the AI works like the speaker of a small parliament: everyone gets a turn, it answers each person by name, and it keeps track of what was agreed. Use a group to brainstorm or to settle a question.

Every person has their own Memory, Plan Book and Feed. The rule behind it: what is shared stays in the group, what is yours stays yours. You carry things across with **Take to my chat**, and nothing private ever flows back into a group.

A short example. Four friends ask in a group which weekend works for a trip. The group's memory keeps the answer: June 21. Sam clicks **Take to my chat** under that message and types "post this to my feed and add booking the cabin to my plan". Now his Feed has the decision, his Plan Book has the task, and his morning agent reminds him the next day.

## Three short walk-throughs

Each one takes a few minutes. The examples are a team, but a club choosing a date or a family planning a trip works the same way.

### 1. Settle a question together

1. Click **New group** in the sidebar, give it a name such as "Next project", and click **Create the group**.
2. Click **Invite people**, then **Create invite link**, and send the link. Whoever opens it and signs in is in the group.
3. Ask your question in the group, for example "Which language should we use for the next project? We know Python and a little Go."
4. Everyone reads the same answer and can reply. The AI answers one message at a time and knows who wrote what.
5. When you agree, say so: "We take Go. Please remember that."

Nobody needs to paste an answer from another AI. If someone does, paste it into the group and ask the AI there to compare.

### 2. Keep track of what was decided

1. In the group, click **Memory** at the top. You see every fact the AI kept, and whose message it came from.
2. Weeks later, anyone can ask in the group: "What did we decide about the language?" The AI looks it up.
3. When a decision changes, tell the AI in the group: "We changed our mind: Python, not Go." It corrects the fact.
4. A fact that is simply wrong can be deleted on the Memory page, by the group's admin or by the person whose message it came from.

Only the AI writes to a group's memory. That is on purpose: nobody can quietly type a "decision" in.

### 3. Stay up to date without looking everywhere

Your **Feed** is a private timeline. Two things fill it.

**Something said in a group:**

1. Open **Feed**, click the settings button, then **Who may post**, and click **Let the AI post**. You do this once.
2. In the group, click the arrow under the message you want, **Take to my chat**.
3. A new chat of your own opens with the message quoted. Type "post this to my feed" and send. Or, with the Plan Book turned on, "add this to my plan for Friday".

**A helper that reports to you:**

1. Click **Agents**, then **New agent**. Describe its job in plain words, for example "Every morning, list what is due in my Plan Book this week and post a short summary to my feed."
2. Choose when it runs, and under **Which tools may it use?** allow the Feed (and the Plan Book, to read it).
3. Read the result in your Feed. If the agent runs into trouble, the Feed tells you once.

The rest of this guide explains each part in detail.

## Start in three steps

1. **Sign in.** Open Knitnox in your browser and log in with the account your admin gave you. If the login page offers **Create an account**, you can make one yourself; you type the password twice.
2. **Type a message** in the box on the home screen and press **Enter**.
3. **Read the answer.** It appears word by word. You can close the tab; the answer keeps being written and is there when you return.

If the home screen says "No models are available yet", ask your admin to add one, or add your own API key under **Settings → Your API keys**.

New accounts see a **Getting started** list on the home screen. It ticks off each step as you do it. Click **Hide** when you no longer need it.

## Find your way around

| Where | What is there |
|---|---|
| **Sidebar, top** | **New chat** and **Search or jump to…** (`Ctrl+K`) |
| **Sidebar, middle** | **Agents**, **Feed**, **Plan Book**, **Memory**, **Settings**, **Help & guides**, then your agents and your chats |
| **Sidebar, bottom** | Your name: switch between light and dark, open Settings, report an issue, log out |
| **A small "?"** | Click it for a plain-words explanation of the thing next to it |

To change how Knitnox looks, open **Settings → Appearance**: choose **Auto**, **Light** or **Dark**, and pick an accent colour. The choice is saved in the browser you are using.

## Chat basics

| I want to… | Do this |
|---|---|
| Start a new chat | Click **New chat** in the sidebar, or press `Ctrl+Shift+O` |
| Choose the AI model | Use the model picker under the message box on the home screen. Change it later by clicking the model name at the top of the chat. In a group only its admin can |
| Write several lines | Press `Shift+Enter` for a new line. `Enter` sends |
| Send while the AI is still answering | Just send. Your message waits in a queue (up to 10) and is answered in order |
| Stop an answer | Click the square **Stop** button. What was written so far is kept |
| Fix my last message | Hover it and click the pencil. The old answer is replaced |
| Get a different answer | Click the circular arrow under the latest answer |
| Copy an answer or a code block | Click the copy icon under the message, or **Copy** on the code block |
| Get a picture | Ask for one in your own chat: "paint a red bicycle by the sea". It appears in the conversation; click it to see it large. Needs a picture model (see below) |
| Have an answer read to me | Click the speaker button under the answer. The first time takes a moment; after that it plays at once. Needs a voice model (see below) |
| See a picture from another website | Click the **Show picture from …** button. Pictures from other sites are not loaded by themselves, so an answer cannot quietly send anything to another site |

On a phone, **Enter** adds a new line and the arrow button sends.

**Pictures and voice need a model.** If your admin shares a picture or voice model, both work for you with nothing to set up. If you added your own model of that kind, it is used when the admin shares none. To choose between them, or to turn one off, open **Feed → Settings → Pictures, voice and video**. If the speaker button is missing, or the AI says it cannot make pictures, no such model exists yet. Pictures are made in your own chats only, not in groups. With the server's models there is a daily limit, shared with the Feed.

## Attach images and files

Click **Attach**, paste from the clipboard, or drag files onto the page. You can attach up to 6 files per message.

- **Images** (PNG, JPEG, WebP, GIF) need a model that can see images. If the model can't, the app tells you.
- **Documents** work with every model: PDF, text, Markdown, CSV, JSON and code files. The text inside is read and given to the AI.

Scanned PDFs without a text layer, and Word, Excel or PowerPoint files, are not read.

## Find and organise chats

- **Find anything:** press `Ctrl+K`, or click **Search or jump to…** in the sidebar. Type a few letters to find a chat, an agent or a page. Move with the arrow keys and press **Enter** to open.
- **Search inside messages:** type your words in the same box and choose **Search inside messages** at the bottom of the list.
- **Pin, archive, rename, export or delete:** click the **⋯** next to a chat.
- **Long chats:** click **Load earlier messages** at the top.

When a chat gets very long, older messages are summarised so the AI can keep going. Nothing is deleted; you can still scroll back and read everything.

## Tell the AI how to behave

- **In all your own chats:** **Settings → Custom instructions**. Example: "Answer in Bengali" or "Keep answers short".
- **In one chat only:** open the chat, click **Share**, and fill in **Instructions for this chat**. In a group it is **People → Group settings**, and only the admin can change it. Your personal instructions are never used in a group.

## Choose what you see of the AI's work

Under **Settings → Appearance** two switches decide how much of the AI's work a chat shows you. They follow your account to every device and change only what you see.

| Switch | At first | On | Off |
|---|---|---|---|
| **Show thinking** | on | Models that share their reasoning show it in a fold-away block | No such block |
| **Show tool details** | off | Every tool the AI uses, and its answer, is a row of its own | One line under your message, like **Used 2 tools**; click it to see what they did |

Some things are always shown: a tool that waits for your permission, a tool that did not work (the line says so), a note while a tool is running, and a picture the AI made. An agent's page always shows everything the agent did.

## Chat with other people: groups

A chat is one of two kinds, and it stays that kind for good:

| Kind | Who writes | What the AI may use |
|---|---|---|
| **Your own chat** | Only you | Your Memory, Plan Book, Feed, pictures, agents and tool servers |
| **A group** | Everyone you invite | Only what belongs to the group: its own memory and its own tool servers |

A chat of your own never turns into a group, and a group never turns into a personal chat. That way nothing your personal tools answered can end up in front of other people.

### Start a group

1. Click **New group** in the sidebar.
2. Give it a name and pick the model that answers. A model under **Yours** uses your own API key: you pay for everyone's messages in that group.
3. Click **Create the group**, then **Invite people**. Choose **Can chat** or **Read only**, click **Create invite link** and send the link. Anyone who opens it and signs in joins.

Everyone's messages go into one queue and the AI answers them one at a time. The AI sees who wrote each message. People who join later can read what was written before.

You are the group's **admin**: only you invite and remove people, change the name, the model and the instructions, add tool servers and delete the group.

### What a group has

- **Its own memory.** The AI saves what the group decides and learns, and looks it up later. Open it with **Memory** at the top of the group. Everyone in the group can read it. **Only the AI writes there**: nobody can add or change a fact, so nobody can plant one. A fact that is wrong can be deleted, by the admin, or by the person whose message it came from.
- **Its own tools.** The admin adds tool servers under **People → The group's tools**, for example a web search. Every member can make the AI use them, so do not add one that acts with your private accounts.
- **One model**, chosen by the admin.

Nobody's personal tools are used in a group, also not the admin's.

### Ask in private about a group

Did you miss something, or would you rather not ask in front of everyone?

1. Click the arrow under a message in the group (**Take to my chat**), or open a new chat and switch the group on under **Tools → Your groups**.
2. Ask there: "What did we decide about the budget, and why?" or "Explain what Leo meant, in simple words."

The AI reads the group's memory and latest messages to answer. Your question and the answer stay in your own chat: nobody in the group sees them, and nothing is written back to the group.

### Use your own tools with something said in a group

Say the group decided something and you want it in your Feed or your Plan Book.

1. Under the message, click **Take to my chat** (the arrow).
2. A new chat of your own opens with the message quoted. It is **connected** to the group: its AI can read the group's memory and latest messages, but cannot write there.
3. Say what to do, for example "post this to my feed".

You can also connect a group without a message: open the **Tools** menu of your chat (or of the home screen) and switch the group on under **Your groups**. If you leave a group or are removed, the chat can no longer read it.

## Show a chat to others: a read-only link

1. Open a chat of your own and click **Share**.
2. Click **Create a read-only link** and send it.

Anyone who has the link can **read** the chat. They need no account and cannot write.

- The link shows the chat **as it was when you made it**. What you write later stays private until you click **Update the link to now**.
- Tool calls and their results are left out unless you tick **Also show what tools did**. Leave it off if your tools answer with private things.
- **Stop sharing** makes the link stop working for everyone.
- An admin can turn read-only links off for the whole server.

Want others to write too? Start a group instead.

## Give the AI tools (MCP servers)

Tools let the AI do things outside the chat, such as search your mail or read a calendar. Tools come from **MCP servers**.

1. Go to **Settings → Tools → Add an MCP server**.
2. Enter a name and the server's URL. Add headers if the server needs a key.
3. Choose what happens when the AI wants to use a tool:
   - **Run it automatically**: fine for tools that only read.
   - **Ask me before running**: best for tools that change things. The chat shows a card with **Allow once**, **Deny** and **Always allow**.
4. Click **Test** to see the server's tools.

Your own chats use all of your enabled servers, unless you switch one off for a chat (next section).

## Choose the tools for one chat

Click **Tools** next to **Attach**, on the home screen before you start or in any chat you own. On a phone the menu opens as a sheet at the bottom of the screen; the cross at its top closes it.

| Row | What it switches |
|---|---|
| **Memory** | Saving facts about you and looking them up |
| **Plan Book** | Adding, listing and finishing your tasks |
| **Feed** | Posting to your feed and reading your posts |
| **Pictures** | Making a picture in the chat |
| **Agents** | Handing work to the agents connected to this chat |
| Each tool server | Every tool of that server |

- A switch changes **this chat only**. Other chats, and the setting itself, stay as they are.
- A greyed-out row is something you have not turned on yet; **Open** takes you to where that is done.
- The button shows how many things are off, for example **Tools · 2 off**.
- Fewer tools make answers cheaper and keep the AI on the point. Switching Memory off is also the way to have one chat the AI remembers nothing from.
- **In the middle of a chat** the menu shows a note: after a change, the next reply is a little slower and costs a little more, once, because the AI provider has to read the conversation again from the start. Choosing before the first message costs nothing.
- **Your groups** at the end of the menu connects this chat to a group you are in, so the AI can read that group's memory and latest messages.
- In a group only the admin sees the button. It lists what belongs to the group: **Group memory** and the group's tool servers. Personal tools are not listed, because a group never uses them.

## Agents: workers that run in the background

An **agent** is a saved worker with a job, its own tools and a to-do list. It can run on a schedule or when asked, and it starts fresh every run.

### Create an agent

1. Click **Agents** in the sidebar, then **New agent**. Not sure where to start? Pick one of the examples at the top and change it.
2. Give it a **name**.
3. Write **what it should do**: its job, in plain words.
4. Choose **when it works**: only when asked, or on a schedule.
5. Click **Create agent**. The button stays at the bottom of the screen.

Those three answers are enough. Two optional boxes sit below them:

- **Tools and access:** let the agent use your Memory, Plan Book or Feed, and tick the tool servers (MCP) it may use. A ticked server shows a box: leave it empty to allow all of its tools, or type only the names it needs; **Show tools** lists them. While the box is closed, it says in one line what you chose.
- **Model and limits:** which AI model it uses, and how many steps and runs it may make.

If something is missing, the page tells you next to the field, and nothing you typed is lost. After creating, you land on the agent's page, where **Add its first task** and **Run it now** are waiting.

### Give it work

Open the agent and click **Tasks & settings**.

- **Add a task:** type a title, add details if needed, and click **Add task**. Tick **Start the agent now** if it should not wait for its schedule.
- **Start a run yourself:** click **Run now**, or type an instruction in the message box.
- **See what it did:** its page shows every run like a chat. Finished tasks show their result in the task list.

### Use an agent from a chat

1. On the home screen, click the **Agents** pill under the message box and tick one or more agents. In an existing chat, use the **Agents** button at the top.
2. Ask in normal words, for example: *"Ask Inbox helper to reply to Sam about the invoice."*
3. The AI hands the task to the agent. When the agent finishes, the result appears in the same chat.

Disconnecting an agent from a chat does not stop the agent.

### Give one agent its own tools

A dedicated server is used by one agent only. Your chats and other agents never see it. It works alongside the shared servers you ticked.

- **While creating the agent:** open **Tools and access**, then **Add a tool server only this agent uses**.
- **Later:** in **Tasks & settings**, find **This agent's own MCP servers**. Add as many as you need, test them, or remove them.

### Pause, limits and memory

- **Pause:** untick **Active** in the agent's settings, or click **Pause** on the Agents page.
- **Limits:** set the steps per run and the automatic runs per day under **Limits**. If the schedule asks for more runs than the daily limit allows (every 15 minutes is 96 a day), a note under the schedule offers to raise it. An agent that used up its runs for the day shows **Daily limit reached** and starts again at midnight UTC; **Run now** still works.
- **At a glance:** next to **Agents** in the sidebar, a number with a blinking dot is how many agents are working right now, and an amber number is how many need a look (the last run failed, or the daily limit is reached). They update by themselves. The Agents page shows the totals.
- **When something goes wrong:** if a run fails, or a tool server cannot be reached (the run then works without its tools), a note stays at the top of the agent's page with a **Run again** button, and the run itself is marked. Knitnox also writes one post in your Feed, marked **Problem**, and one more, **Working again**, when a run succeeds. It does not post after every failed run, and these posts do not count toward the agent's posts per day.
- **After the server was off:** a scheduled agent that missed its time runs once, shortly after the server starts, then continues on its schedule from there. Missed runs are not made up.
- **Adjustments:** short extra guidance on top of the instructions. A connected chat's AI may edit this part, never the instructions or tools. Every change is listed.
- **Its own notes:** notes the agent writes for its later runs. You can clear them.

### Let an agent use your Memory, Plan Book and Feed

An agent gets none of these unless you allow it. You choose them under **Built into Knitnox**: in **Tools and access** while creating the agent, or later in its **Setup** tab under **Which tools may it use?** (then **Save changes**).

- **Memory:** tick it to let the agent read what Knitnox remembers about you. It can never change it.
- **Plan Book:** choose whether the agent may read your tasks, or also add, change and delete them.
- **Feed:** tick it to let the agent post to your feed, and set how many posts a day it may add.

### The agent store

The **Agent store** tab has ready-made agents shared by people on your server.

- **Install one:** open it, read its guide and the tools it needs, then click **Add to my agents**. Your copy starts paused. Pick its model and your own tools, tick **Active**, and save.
- **Share yours:** open your agent, go to **Share in the agent store**, write a description and a short guide, and click **Publish**.

Sharing copies only the name, instructions, schedule and limits. Your tasks, memory, runs and MCP servers are never shared.

## Feed: short posts in one place

The **Feed** is one page of short posts, newest first. It is where you see at a glance what happened and what was found, without opening every chat and agent. Open **Feed** in the sidebar. The number next to it is how many posts you have not looked at yet.

The page shows only posts. Everything else is one click away in the row at the top.

| At the top | What it does |
|---|---|
| **Latest / Mine / Shared** | Which posts you see (see Views below) |
| **Everyone ▾** | Show only posts written by people, by the AI, by agents, or by one agent |
| **Find a post…** | Search the text of posts and reports |
| **Settings** (the sliders button) | Who may post, the models for pictures, voice and video, how the Feed works, and the bin |
| **New post** | Opens the box to write a post |

### Write and read posts

- **Post:** click **New post**, type, and click **Post**. A post is at most 500 characters.
- **Add a longer report:** open **Add a longer report** and write the details. They open behind **Read the full report**.
- **Let AI add:** when a picture, voice or video model is set up, you can ask for **A picture**, **Read it aloud** or **A video** with your post. The post appears at once; the picture, recording or video shows up on it by itself when it is ready.
- **Who can see it:** choose **Only me** or **Everyone on this server**. You can change this later from the post's **⋯** menu.
- **Older posts:** the page shows the newest 20. Scroll down and 10 more load each time you reach the end.
- **New posts:** when a post arrives while you are reading, a button such as **3 new posts** appears at the top. Click it to see them. Nothing moves until you do.

### Views

| View | What it shows |
|---|---|
| **Latest** | Your own posts and what others shared |
| **Mine** | Posts on your own feed, private and shared |
| **Shared** | Posts that people on this server shared with everyone |
| **Bin** | Posts you deleted, kept for 30 days. Open it from the settings button |

### Change, share and delete

Open the **⋯** menu on one of your posts.

- **Change the text:** edit the post and its report in place.
- **Share with everyone / Make it private again:** a shared post shows **Seen by N people**, the number of other people who were shown it.
- **Delete:** the post goes to the bin. Click **Undo** right away, or restore it later from the **Bin**.

There are no likes, comments or follows.

### Let the AI and agents post

Open the settings button, then **Who may post**. The AI cannot post until you click **Let the AI post**. After that, in your own chats:

- "Post a short summary of this to my feed."
- "Post this with a diagram of the steps."
- "What is on my feed about the invoice?"

To let an agent post, open the agent, go to **Setup** → **Which tools may it use?**, tick **Feed**, set its posts per day and save. When the limit is reached, the agent keeps working but posts nothing more that day.

What the AI and agents post is always private. Only you can share a post. They can read your own posts but never what other people shared. The feed is never used in groups; to post something said in a group, click **Take to my chat** under the message.

### What a post can carry

| Extra | Needs a model? | What it is |
|---|---|---|
| Report | No | A longer text behind **Read the full report** |
| Chart | No | Bars or a line, drawn from a few numbers the AI gives |
| Diagram | No | A picture the AI draws itself, for steps, timelines and how things connect |
| Picture | A picture model | An image made from a description |
| Voice | A voice model | The text read aloud |
| Video | A video model | A short film made from a description. It takes minutes |

When the AI or an agent asks for a picture and its own model can see images, it looks at the finished picture once. If the picture does not show what was asked for, it is drawn a second time and the post says so.

### Pictures, voice and video: choosing the models

Open the settings button, then **Pictures, voice and video**.

- **Pick a model for each kind**, or choose **Off**. "The server's model" is the one your admin shares with everyone.
- **Add my own model:** if you have your own account with an AI provider, type the model name, the provider's address and your key. The model is used for your feed right away. It is also listed under **Settings → Your API keys**, so you never type a key twice.
- **Limits:** with the server's models you can make a set number of pictures, recordings and videos per day (your admin chooses the number). Your own models are not limited.

The same small form is on the **Memory → Settings** page, to add your own embedding model for smart search.

## Plan Book: your tasks and projects

The **Plan Book** is your own to-do list. Open **Plan Book** in the sidebar. The number next to it is how many tasks are due today or late.

The page has three tabs: **Tasks** (your list), **Projects** and **Settings**.

### Add and finish tasks

Above your tasks you see today's date, one sentence about what is due, and the next seven days. The small number on a day is how many tasks are due then.

| I want to… | Do this |
|---|---|
| Add a task | Type it in the **Add a task** box and press **Enter** |
| Give it a date, a project or a note | Click in the box: the choices open under it. Pick **Today**, **Tomorrow**, **Next week** (the coming Monday) or **Pick a date…** before adding |
| Finish a task | Click the circle in front of it |
| Take back a wrong click | Click **Undo** in the line that appears after you add, finish, move or delete a task |
| Move a task to another day | Click its date (or **Add a date**) and choose **Today**, **Tomorrow**, **Next week** or **Another date…** |
| Catch up on late tasks | Next to **Late**, click **Move all to today** |
| See one day only | Click that day in the row of seven days. Click it again, or **Show everything**, to go back |
| Change a task, or see who changed it | Click its text, or **⋯** then **Change, or see its history** |
| Show that I am working on it | **⋯**, then **I am working on it** |
| Delete a task | **⋯**, then **Delete**. It waits in the bin for 30 days |
| Bring a deleted task back | Click the bin button next to **Find a task…**, then **Restore** |
| See what is urgent | **Today** shows tasks due today or late |
| Find a task | Type in **Find a task…** |

### Projects

A **project** groups tasks that belong together, like "Move house".

- **Create one:** open the **Projects** tab, click **New project**, give it a name and, if you like, a line about its goal.
- **Look at one:** click it on the **Projects** tab, or pick it in the **All tasks** menu above your list. You see its tasks and how many are done.
- **Rename or delete:** open the project, then **Rename or delete**. Deleting a project moves its tasks to the bin.

### Let the AI and agents help

The AI cannot see your Plan Book until you click **Let the AI use it** (on the banner, or on the **Settings** tab, where you can also turn it off again). After that, in your own chats:

- "Add a task to call Sam on Friday."
- "What is on my plan today?"
- "Mark the passport task as done."

The Plan Book is never used in groups; to add something said in a group, click **Take to my chat** under the message. To let an agent use it, open the agent, go to **Setup** → **Which tools may it use?** → **Plan Book**, and choose **Read them** or **Read, add, change and delete them**. Tasks added by the AI or an agent are labelled, and anything they delete goes to the bin.

## Memory: what the AI remembers about you

Without memory, every chat starts from zero. With it, the AI saves short facts you share and looks them up later, so you do not repeat yourself. Open **Memory** in the sidebar.

### Turn it on

Memory is **off** until you click **Turn on memory**. You can turn it off again under **Memory → Settings**. Turning it off deletes nothing.

| Where | What happens |
|---|---|
| Your own chats | The AI can save facts and look them up |
| Groups | Your memory is never used. A group has a separate memory of its own |
| Agents | Only an agent you allowed, and it can only read |

### Use it in a chat

- **Save something:** just tell the AI, or say "remember that I am vegetarian". It saves a short note and says so.
- **Use it later:** ask as usual. The AI searches its notes before it answers.
- **Fix it:** say "that is wrong, I moved to Lisbon" and it corrects the note, or change it yourself on the Memory page.

### Look after your memories

| I want to… | Do this |
|---|---|
| See everything | **Memory → Memories**. Newest first |
| Find one | Type in **Search your memory** |
| Add one myself | **Add a memory yourself** |
| Change one | Click the pencil on its card |
| Make sure it is never forgotten | Click the pin. Pinned memories are shown to the AI in every chat. Keep them few |
| Delete one | Click the bin |
| Delete everything, or download a copy | **Memory → Settings → Your data** |

### Things and connections

A **thing** is a person, place or project your memories name. A **connection** says how two things belong together, for example "Sam works at Acme Bakery".

- Things appear by themselves when a memory names them. When you add a memory, fill in **Who or what is this about?**
- To connect two things, open **Connect things to each other** in the form and fill the three boxes: a thing, how it relates, another thing.
- **Memory → Things & connections** lists every thing. Open one to see all that is known about it.
- The **map** shows things as circles. Click a circle to bring in everything it is linked to.
- Is one thing listed under two names? Open it, choose **Rename or merge**, and give it the other's exact name.

### Smart search

Memory always finds notes that share words with your question. With an **embedding model** it also finds notes that mean the same in other words: "what do I eat?" finds "I am vegetarian".

- Your admin can set one for everyone. **Memory → Settings** shows whether it is on.
- To use your own, add a provider under **Settings → Your API keys**, then pick it under **Memory → Settings** and type the model name, for example `text-embedding-3-small`. Click **Test it** first.

## Notifications and installing as an app

- **Notifications:** **Settings → Notifications → Enable on this device**. You are notified when an answer finishes while you are away, or when a tool needs your approval.
- **Install as an app:** use your browser's "Install" or "Add to Home Screen" option. On iPhone, add it to the Home Screen first, then enable notifications.

Both need the site to be served over `https://`.

## Your account

Under **Settings** you can change your username and your password (the current password is asked for, and a new password is typed twice), see how much of your token allowance you have used, add your own API keys, and **Log out everywhere**. Changing your password signs you out on your other devices.

**Forgot your password or username?** Ask an administrator of your Knitnox: they can tell you your username and set a new password for you under **Admin → Users**. There is no e-mail reset.

Only shared models count toward your allowance. Your own keys are never limited.

**Time zone.** The AI is told your local date and time, so "tomorrow" and "this evening" mean what you expect. Knitnox reads the time zone from your device. To fix it yourself, type it under **Settings → Account** (for example `Europe/Lisbon`) and untick **Set it from my device**. Group chats go by the server's clock, because people may be in different time zones.

## For admins

The first account created is the admin. Admins see **Admin panel** in the sidebar. After the first account, nobody else can sign up until you allow it: create accounts under **Users**, or tick **Allow anyone to sign up** under **Settings**.

| Tab | What you do there |
|---|---|
| **Providers** | Add shared AI providers and choose their models |
| **Users** | Create accounts, set token limits, rename, set a new password, disable or delete users |
| **Usage** | See tokens and cost per user and model |
| **Audit** | See who changed what in the admin panel |
| **Settings** | Sign-up, queue size, step and time limits, backups, housekeeping, the system prompt and more |

**Backups.** Knitnox makes a backup every day and keeps the newest seven. **Admin panel → Settings → Backups** shows when the last one ran and warns if it failed; **Back up now** makes one at once. Ask whoever runs the server to keep the backup folder on another disk.

**Someone forgot their password or username.** Open **Users**, click **⋯** next to the person, and type a new password or a new username. If you forgot your own, it is reset on the computer where Knitnox runs (`knitnox list-users`, `knitnox reset-password NAME NEW-PASSWORD`).

**Housekeeping.** Old usage and audit lines and old agent runs are removed after the limits under **Settings → Housekeeping**. Chats, memories, tasks and posts are never removed this way.

**Memory for everyone.** Under **Admin panel → Settings → Long-term memory**, pick a shared provider and type the name of its embedding model to give every user smart search. Click **Test it** before saving. Without it, memory still works with word search.

## When something goes wrong

| What you see | What to do |
|---|---|
| "No model selected" or "model is disabled" | Click the model name at the top of the chat and pick another one. In a group, ask its admin |
| "Token quota exceeded" | Wait for the next period, ask an admin, or use your own API key |
| "Waiting for a free slot…" | Other chats are being answered first. It starts by itself |
| "Queue is full" | Wait for the AI to catch up, or remove a queued message |
| A tool card is waiting | Click **Allow once** or **Deny**. Unanswered cards are denied after a while |
| An agent does nothing | Check that it is **Active**, has a model, and has tools ticked |
| An answer stopped half way | Click **Retry** under it |
| "The provider stopped answering" or "the answer was cut off" | The AI provider went silent or hung up. What was written is kept. Click **Retry** |
| "Stopped after … minutes" | The answer took longer than the server allows. Try a shorter request, or ask an admin to raise the limit |
| "Sign-up is closed" | Ask an admin for an account |
| "Too many failed attempts" | Wait 15 minutes, or ask an admin to set a new password |
| Pages take very long to open | If you have many Knitnox tabs open, close some |

## Report a problem or an idea

Open **Help → Issues** and click **Report an issue**. Say what you did, what you expected and what happened. Everyone on your server can read issues, so never paste passwords or keys.
