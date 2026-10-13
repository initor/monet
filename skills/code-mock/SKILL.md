---
name: code-mock
description: Use when the user asks to practice coding, run a coding mock, or drill coding problems. Runs Java-focused coding mock interviews or drills specific aspects (clarification, optimization, edge cases) with a realistic Staff+ interviewer, evaluates against a coding rubric, generates golden solutions, and auto-records sessions.
---

## Preconditions

- Determine mode from user input: **mock** (full flow: clarify -> code -> probe -> variation -> evaluation) or **drill** (focused practice on specific aspects like optimization, clarification, or edge case discussion).
  - "run a coding mock" / "practice coding" / "code mock" -> mock mode
  - "drill optimization" / "practice edge cases" / "code drill" -> drill mode
  - If ambiguous, ask user to clarify: "Do you want to run a full coding mock, or drill a specific aspect (clarification, optimization, edge cases, code quality)?"
- If user provides a topic/category filter (e.g., "give me a graph problem"), apply it in problem selection (Step 1).
- Record `started_at` timestamp (ISO 8601) at session start. This is used for session recording in Step 6.

## Step 1: Select problem and confirm scope

### Problem bank

Select from the curated bank below. Each problem is medium difficulty (LC medium equivalent). Problems are organized by category for balanced coverage.

**Selection logic:**
- If user specifies a category (e.g., "give me a graph problem"), pick randomly from that category.
- If no category filter, pick uniformly random across all categories.
- Never repeat a problem within the same session.

#### Arrays & Strings

**1. Two Sum Variants**
- Description: Given an array of integers and a target sum, find all unique pairs of numbers that add up to the target. Return the pairs sorted in ascending order.
- Format: Method signature provided
- Signature: `List<int[]> findPairs(int[] nums, int target)`
- Example: `nums = [1, 3, 2, 2, 3, 4], target = 5` -> `[[1,4], [2,3]]`
- Constraints: Array length 0 to 10^5, values -10^9 to 10^9, duplicates possible
- Test cases:
  1. `nums = [1, 3, 2, 2, 3, 4], target = 5` -> `[[1,4], [2,3]]`
  2. `nums = [], target = 5` -> `[]`
  3. `nums = [5], target = 5` -> `[]`
  4. `nums = [0, 0, 0], target = 0` -> `[[0,0]]`
  5. `nums = [-1, 2, -3, 4], target = 1` -> `[[-3,4], [-1,2]]`

**2. Longest Substring Without Repeating Characters**
- Description: Given a string, find the length of the longest substring that contains no repeating characters.
- Format: Method signature provided
- Signature: `int lengthOfLongestSubstring(String s)`
- Example: `s = "abcabcbb"` -> `3` (substring "abc")
- Constraints: String length 0 to 5 * 10^4, consists of English letters, digits, symbols, spaces
- Test cases:
  1. `s = "abcabcbb"` -> `3`
  2. `s = "bbbbb"` -> `1`
  3. `s = ""` -> `0`
  4. `s = "pwwkew"` -> `3`
  5. `s = " "` -> `1`

**3. Product of Array Except Self**
- Description: Given an integer array, return an array where each element is the product of all elements except itself. Solve without using division and in O(n) time.
- Format: Method signature provided
- Signature: `int[] productExceptSelf(int[] nums)`
- Example: `nums = [1, 2, 3, 4]` -> `[24, 12, 8, 6]`
- Constraints: Array length 2 to 10^5, values -30 to 30, product of any prefix/suffix fits in 32-bit integer
- Test cases:
  1. `nums = [1, 2, 3, 4]` -> `[24, 12, 8, 6]`
  2. `nums = [-1, 1, 0, -3, 3]` -> `[0, 0, 9, 0, 0]`
  3. `nums = [0, 0]` -> `[0, 0]`
  4. `nums = [2, 3]` -> `[3, 2]`
  5. `nums = [1, 1, 1, 1]` -> `[1, 1, 1, 1]`

#### Hashmaps & Sets

**4. Group Anagrams**
- Description: Given an array of strings, group the anagrams together. An anagram is a word formed by rearranging the letters of another word using all original letters exactly once.
- Format: Description only -- user defines the interface
- Example: `["eat","tea","tan","ate","nat","bat"]` -> `[["eat","tea","ate"],["tan","nat"],["bat"]]`
- Constraints: Array length 1 to 10^4, string length 0 to 100, lowercase English letters only
- Test cases:
  1. `["eat","tea","tan","ate","nat","bat"]` -> groups: `[["eat","tea","ate"],["tan","nat"],["bat"]]`
  2. `[""]` -> `[[""]]`
  3. `["a"]` -> `[["a"]]`
  4. `["abc","bca","cab","xyz","zyx"]` -> groups: `[["abc","bca","cab"],["xyz","zyx"]]`
  5. `["listen","silent","hello"]` -> groups: `[["listen","silent"],["hello"]]`

**5. Subarray Sum Equals K**
- Description: Given an array of integers and an integer k, find the total number of contiguous subarrays whose sum equals k.
- Format: Method signature provided
- Signature: `int subarraySum(int[] nums, int k)`
- Example: `nums = [1, 1, 1], k = 2` -> `2`
- Constraints: Array length 1 to 2 * 10^4, values -1000 to 1000, k from -10^7 to 10^7
- Test cases:
  1. `nums = [1, 1, 1], k = 2` -> `2`
  2. `nums = [1, 2, 3], k = 3` -> `2`
  3. `nums = [1], k = 0` -> `0`
  4. `nums = [-1, -1, 1], k = 0` -> `1`
  5. `nums = [0, 0, 0], k = 0` -> `6`

#### Trees & Graphs

**6. Validate Binary Search Tree**
- Description: Given the root of a binary tree, determine if it is a valid binary search tree (BST). A valid BST has: left subtree nodes strictly less than root, right subtree nodes strictly greater, and both subtrees are also valid BSTs.
- Format: Method signature provided
- Signature: `boolean isValidBST(TreeNode root)`
- Example: `root = [2, 1, 3]` -> `true`
- Constraints: Number of nodes 1 to 10^4, node values -2^31 to 2^31 - 1
- Test cases:
  1. `root = [2, 1, 3]` -> `true`
  2. `root = [5, 1, 4, null, null, 3, 6]` -> `false`
  3. `root = [1]` -> `true`
  4. `root = [2, 2, 2]` -> `false` (not strictly less/greater)
  5. `root = [Integer.MIN_VALUE]` -> `true`

**7. Number of Islands**
- Description: Given a 2D grid of '1's (land) and '0's (water), count the number of islands. An island is surrounded by water and is formed by connecting adjacent land cells horizontally or vertically.
- Format: Method signature provided
- Signature: `int numIslands(char[][] grid)`
- Example: `grid = [["1","1","0"],["0","1","0"],["0","0","1"]]` -> `2`
- Constraints: Grid dimensions 1 to 300, cells are '0' or '1'
- Test cases:
  1. `[["1","1","1"],["0","1","0"],["1","1","1"]]` -> `1`
  2. `[["1","0","1"],["0","0","0"],["1","0","1"]]` -> `4`
  3. `[["0"]]` -> `0`
  4. `[["1"]]` -> `1`
  5. `[["1","1"],["1","1"]]` -> `1`

**8. Course Schedule (Cycle Detection)**
- Description: There are a total of numCourses courses labeled 0 to numCourses-1. Given an array of prerequisites where prerequisites[i] = [a, b] means you must take course b before course a, determine if it is possible to finish all courses.
- Format: Description only -- user defines the interface
- Example: `numCourses = 2, prerequisites = [[1,0]]` -> `true`
- Constraints: numCourses 1 to 2000, prerequisites length 0 to 5000, no duplicate edges
- Test cases:
  1. `numCourses = 2, prerequisites = [[1,0]]` -> `true`
  2. `numCourses = 2, prerequisites = [[1,0],[0,1]]` -> `false`
  3. `numCourses = 3, prerequisites = []` -> `true`
  4. `numCourses = 4, prerequisites = [[1,0],[2,1],[3,2]]` -> `true`
  5. `numCourses = 3, prerequisites = [[0,1],[1,2],[2,0]]` -> `false`

#### Dynamic Programming

**9. Coin Change**
- Description: Given an array of coin denominations and a total amount, find the fewest number of coins needed to make up that amount. If that amount cannot be made, return -1.
- Format: Method signature provided
- Signature: `int coinChange(int[] coins, int amount)`
- Example: `coins = [1, 5, 10, 25], amount = 30` -> `2` (25 + 5)
- Constraints: Coin count 1 to 12, coin values 1 to 2^31 - 1, amount 0 to 10^4
- Test cases:
  1. `coins = [1, 5, 10, 25], amount = 30` -> `2`
  2. `coins = [2], amount = 3` -> `-1`
  3. `coins = [1], amount = 0` -> `0`
  4. `coins = [1, 2, 5], amount = 11` -> `3`
  5. `coins = [186, 419, 83, 408], amount = 6249` -> `20`

**10. Longest Increasing Subsequence**
- Description: Given an integer array, find the length of the longest strictly increasing subsequence. A subsequence is derived by deleting some or no elements without changing the order.
- Format: Method signature provided
- Signature: `int lengthOfLIS(int[] nums)`
- Example: `nums = [10, 9, 2, 5, 3, 7, 101, 18]` -> `4` (subsequence [2, 3, 7, 101])
- Constraints: Array length 1 to 2500, values -10^4 to 10^4
- Test cases:
  1. `nums = [10, 9, 2, 5, 3, 7, 101, 18]` -> `4`
  2. `nums = [0, 1, 0, 3, 2, 3]` -> `4`
  3. `nums = [7, 7, 7, 7]` -> `1`
  4. `nums = [1]` -> `1`
  5. `nums = [1, 2, 3, 4, 5]` -> `5`

**11. Decode Ways**
- Description: A message containing letters A-Z is encoded where A=1, B=2, ..., Z=26. Given a string of digits, determine the total number of ways to decode it.
- Format: Description only -- user defines the interface
- Example: `s = "226"` -> `3` (BZ=2-26, VF=22-6, BBF=2-2-6)
- Constraints: String length 1 to 100, contains only digits, no leading zeros in valid decodings
- Test cases:
  1. `s = "226"` -> `3`
  2. `s = "12"` -> `2`
  3. `s = "06"` -> `0`
  4. `s = "0"` -> `0`
  5. `s = "11106"` -> `2`

#### Linked Lists

**12. Merge K Sorted Lists**
- Description: Given an array of k sorted linked lists, merge all lists into one sorted linked list.
- Format: Method signature provided
- Signature: `ListNode mergeKLists(ListNode[] lists)`
- Example: `lists = [[1,4,5],[1,3,4],[2,6]]` -> `[1,1,2,3,4,4,5,6]`
- Constraints: k from 0 to 10^4, total nodes 0 to 10^4, node values -10^4 to 10^4, each list sorted ascending
- Test cases:
  1. `lists = [[1,4,5],[1,3,4],[2,6]]` -> `[1,1,2,3,4,4,5,6]`
  2. `lists = []` -> `[]`
  3. `lists = [[]]` -> `[]`
  4. `lists = [[1],[2],[3]]` -> `[1,2,3]`
  5. `lists = [[-2,-1,0],[0,1,2]]` -> `[-2,-1,0,0,1,2]`

**13. Detect Cycle and Find Entry Point**
- Description: Given the head of a linked list, determine if the list has a cycle. If it does, return the node where the cycle begins. If there is no cycle, return null.
- Format: Method signature provided
- Signature: `ListNode detectCycle(ListNode head)`
- Example: List `[3,2,0,-4]` with tail connecting to node at index 1 -> node at index 1 (value 2)
- Constraints: Number of nodes 0 to 10^4, node values -10^5 to 10^5
- Test cases:
  1. `[3,2,0,-4]` tail connects to index 1 -> node with value `2`
  2. `[1,2]` tail connects to index 0 -> node with value `1`
  3. `[1]` no cycle -> `null`
  4. `[]` -> `null`
  5. `[1,2,3,4,5]` tail connects to index 2 -> node with value `3`

#### Stacks & Queues

**14. Min Stack**
- Description: Design a stack that supports push, pop, top, and retrieving the minimum element -- all in O(1) time.
- Format: Description only -- user defines the full class
- Example: `push(-2), push(0), push(-3), getMin() -> -3, pop(), top() -> 0, getMin() -> -2`
- Constraints: Values -2^31 to 2^31 - 1, methods called at most 3 * 10^4 times, pop/top/getMin always called on non-empty stack
- Test cases:
  1. `push(1), push(2), getMin()` -> `1`
  2. `push(-2), push(0), push(-3), getMin(), pop(), getMin()` -> `-3`, `-2`
  3. `push(0), push(1), push(0), getMin(), pop(), getMin()` -> `0`, `0`
  4. `push(Integer.MAX_VALUE), getMin()` -> `Integer.MAX_VALUE`
  5. `push(1), push(1), pop(), getMin()` -> `1`

**15. Daily Temperatures**
- Description: Given an array of daily temperatures, return an array where each element tells how many days you need to wait for a warmer temperature. If there is no future warmer day, put 0.
- Format: Method signature provided
- Signature: `int[] dailyTemperatures(int[] temperatures)`
- Example: `temperatures = [73, 74, 75, 71, 69, 72, 76, 73]` -> `[1, 1, 4, 2, 1, 1, 0, 0]`
- Constraints: Array length 1 to 10^5, temperature values 30 to 100
- Test cases:
  1. `[73, 74, 75, 71, 69, 72, 76, 73]` -> `[1, 1, 4, 2, 1, 1, 0, 0]`
  2. `[30, 40, 50, 60]` -> `[1, 1, 1, 0]`
  3. `[30, 60, 90]` -> `[1, 1, 0]`
  4. `[90, 80, 70, 60]` -> `[0, 0, 0, 0]`
  5. `[55]` -> `[0]`

#### Sliding Window & Two Pointers

**16. Minimum Window Substring**
- Description: Given two strings s and t, find the minimum window substring of s that contains all characters of t (including duplicates). If no such substring exists, return the empty string.
- Format: Method signature provided
- Signature: `String minWindow(String s, String t)`
- Example: `s = "ADOBECODEBANC", t = "ABC"` -> `"BANC"`
- Constraints: s length 1 to 10^5, t length 1 to 10^5, s and t consist of uppercase and lowercase English letters
- Test cases:
  1. `s = "ADOBECODEBANC", t = "ABC"` -> `"BANC"`
  2. `s = "a", t = "a"` -> `"a"`
  3. `s = "a", t = "aa"` -> `""`
  4. `s = "aa", t = "aa"` -> `"aa"`
  5. `s = "abc", t = "cba"` -> `"abc"`

**17. Container With Most Water**
- Description: Given an array of heights where each element represents a vertical line on the x-axis, find two lines that together with the x-axis form a container that holds the most water.
- Format: Method signature provided
- Signature: `int maxArea(int[] height)`
- Example: `height = [1, 8, 6, 2, 5, 4, 8, 3, 7]` -> `49`
- Constraints: Array length 2 to 10^5, height values 0 to 10^4
- Test cases:
  1. `[1, 8, 6, 2, 5, 4, 8, 3, 7]` -> `49`
  2. `[1, 1]` -> `1`
  3. `[4, 3, 2, 1, 4]` -> `16`
  4. `[1, 2, 1]` -> `2`
  5. `[0, 2]` -> `0`

#### Heaps & Priority Queues

**18. Top K Frequent Elements**
- Description: Given an integer array and an integer k, return the k most frequent elements. The answer may be returned in any order.
- Format: Method signature provided
- Signature: `int[] topKFrequent(int[] nums, int k)`
- Example: `nums = [1, 1, 1, 2, 2, 3], k = 2` -> `[1, 2]`
- Constraints: Array length 1 to 10^5, k from 1 to number of unique elements, answer is guaranteed unique
- Test cases:
  1. `nums = [1, 1, 1, 2, 2, 3], k = 2` -> `[1, 2]`
  2. `nums = [1], k = 1` -> `[1]`
  3. `nums = [3, 0, 1, 0], k = 1` -> `[0]`
  4. `nums = [1, 2, 3, 4], k = 4` -> `[1, 2, 3, 4]`
  5. `nums = [-1, -1, 2, 2, 3], k = 2` -> `[-1, 2]`

**19. Find Median from Data Stream**
- Description: Design a data structure that supports adding integers from a data stream and finding the median of all elements added so far.
- Format: Description only -- user defines the full class
- Example: `addNum(1), addNum(2), findMedian() -> 1.5, addNum(3), findMedian() -> 2.0`
- Constraints: Values -10^5 to 10^5, at most 5 * 10^4 calls, findMedian called only after at least one addNum
- Test cases:
  1. `addNum(1), findMedian()` -> `1.0`
  2. `addNum(1), addNum(2), findMedian()` -> `1.5`
  3. `addNum(1), addNum(2), addNum(3), findMedian()` -> `2.0`
  4. `addNum(-1), addNum(-2), findMedian()` -> `-1.5`
  5. `addNum(5), addNum(5), addNum(5), findMedian()` -> `5.0`

#### Backtracking

**20. Generate Parentheses**
- Description: Given n pairs of parentheses, generate all combinations of well-formed parentheses.
- Format: Method signature provided
- Signature: `List<String> generateParenthesis(int n)`
- Example: `n = 3` -> `["((()))","(()())","(())()","()(())","()()()"]`
- Constraints: n from 1 to 8
- Test cases:
  1. `n = 3` -> `["((()))","(()())","(())()","()(())","()()()"]`
  2. `n = 1` -> `["()"]`
  3. `n = 2` -> `["(())","()()"]`
  4. `n = 4` -> 14 valid combinations

**21. Word Search**
- Description: Given a 2D board of characters and a word, determine if the word exists in the grid. The word can be constructed from letters of sequentially adjacent cells (horizontal or vertical neighbors), and the same cell may not be used more than once.
- Format: Method signature provided
- Signature: `boolean exist(char[][] board, String word)`
- Example: `board = [["A","B","C","E"],["S","F","C","S"],["A","D","E","E"]], word = "ABCCED"` -> `true`
- Constraints: Board dimensions 1 to 6, word length 1 to 15, board and word consist of uppercase/lowercase English letters
- Test cases:
  1. `board = [["A","B","C","E"],["S","F","C","S"],["A","D","E","E"]], word = "ABCCED"` -> `true`
  2. Same board, `word = "SEE"` -> `true`
  3. Same board, `word = "ABCB"` -> `false`
  4. `board = [["a"]], word = "a"` -> `true`
  5. `board = [["A","B"],["C","D"]], word = "ABDC"` -> `true`

#### Intervals & Sorting

**22. Merge Intervals**
- Description: Given an array of intervals where intervals[i] = [start, end], merge all overlapping intervals and return the non-overlapping intervals that cover all input intervals.
- Format: Method signature provided
- Signature: `int[][] merge(int[][] intervals)`
- Example: `intervals = [[1,3],[2,6],[8,10],[15,18]]` -> `[[1,6],[8,10],[15,18]]`
- Constraints: Array length 1 to 10^4, interval values 0 to 10^4
- Test cases:
  1. `[[1,3],[2,6],[8,10],[15,18]]` -> `[[1,6],[8,10],[15,18]]`
  2. `[[1,4],[4,5]]` -> `[[1,5]]`
  3. `[[1,4],[0,4]]` -> `[[0,4]]`
  4. `[[1,4],[2,3]]` -> `[[1,4]]`
  5. `[[1,1]]` -> `[[1,1]]`

**23. Meeting Rooms II (Minimum Conference Rooms)**
- Description: Given an array of meeting time intervals where intervals[i] = [start, end], find the minimum number of conference rooms required to host all meetings.
- Format: Description only -- user defines the interface
- Example: `intervals = [[0,30],[5,10],[15,20]]` -> `2`
- Constraints: Array length 1 to 10^4, interval values 0 to 10^6
- Test cases:
  1. `[[0,30],[5,10],[15,20]]` -> `2`
  2. `[[7,10],[2,4]]` -> `1`
  3. `[[0,5],[5,10]]` -> `1` (end time equals start time -- no overlap)
  4. `[[1,5],[2,6],[3,7],[4,8]]` -> `4`
  5. `[[1,10]]` -> `1`

**24. Task Scheduler**
- Description: Given an array of CPU tasks (represented by characters A-Z) and a cooldown interval n, find the minimum number of intervals the CPU will take to complete all tasks. The CPU can either execute a task or be idle in each interval. Identical tasks must be separated by at least n intervals.
- Format: Method signature provided
- Signature: `int leastInterval(char[] tasks, int n)`
- Example: `tasks = ['A','A','A','B','B','B'], n = 2` -> `8` (A B idle A B idle A B)
- Constraints: Tasks length 1 to 10^4, n from 0 to 100, tasks are uppercase English letters
- Test cases:
  1. `tasks = ['A','A','A','B','B','B'], n = 2` -> `8`
  2. `tasks = ['A','A','A','B','B','B'], n = 0` -> `6`
  3. `tasks = ['A','A','A','A','A','A','B','C','D','E','F','G'], n = 2` -> `16`
  4. `tasks = ['A'], n = 5` -> `1`
  5. `tasks = ['A','B','C','D'], n = 3` -> `4`

### Scope confirmation

**For mock mode:** Print the problem title and confirm: "Full coding mock (clarify -> code -> probe -> follow-up -> evaluation). Ready?"

**For drill mode:** Print the problem title and which aspect to drill: "Drill: [aspect] on [problem title]. We'll focus specifically on [what the drill covers]. Ready?"

**Example output:**

```
Problem: Merge K Sorted Lists
Category: Linked Lists
Mode: Full coding mock (clarify -> code -> probe -> follow-up -> evaluation)

Let's begin. Take a moment to read the problem, then start by asking clarifying questions.
```

## Step 2: Clarification phase (both modes)

This phase must feel like a real Staff+ interview. The interviewer presents the problem and expects the candidate to drive the clarification.

### Problem presentation

Present the problem description to the user including:
- Problem description (2-3 sentences)
- Example input/output
- Do NOT reveal the method signature yet for description-only problems (force the user to think about the interface)
- For method-signature problems, show the signature as context

### Clarification expectations

Prompt the user to ask clarifying questions before coding:

```
Take a moment to read the problem. Before you start coding, what clarifying questions do you have?
```

**Good clarifications to reward:**
- Input constraints (size limits, value ranges)
- Edge cases (empty input, single element, null, duplicates)
- Expected behavior on boundary conditions
- Time/space requirements or preferences
- Input format and output format specifics
- Whether in-place modification is acceptable
- Thread safety requirements (if applicable)

**Interviewer rules during clarification:**
- DO NOT answer with the solution approach. Never say "you should use a hashmap" or "think about sorting first."
- Respond to clarifications factually: "Yes, the input can be empty", "Assume 32-bit integers", "Duplicates are possible."
- If the user asks a clarification that reveals the solution approach (e.g., "Should I use a heap?"), respond neutrally: "I'm interested in YOUR approach. What are you considering and why?"
- After the user signals readiness to code (or after 3-4 good clarifications), transition to Step 3.

### Drill mode: clarification drill

For drill mode focused on **clarification**:
- Go deeper: push for 5-7 clarifying questions before allowing the user to stop.
- After each clarification, probe further: "What else? What about [edge case the user missed]?"
- Evaluate clarification quality explicitly (are they asking the RIGHT questions?).
- After thorough clarification, evaluate and skip to Step 5 (no coding).

## Step 3: Coding phase (mock mode and drill:optimization/edge-cases/code-quality)

### Playground generation

Generate the coding playground automatically: invoke `monet:code-playground` with the Skill tool, and pass it this problem context as its arguments:

- **Title:** [problem title]
- **Description:** [problem description]
- **Method signature:** [signature or "User-defined interface"]
- **Test cases:** [list the test cases]

Follow the loaded `monet:code-playground` instructions to produce the playground HTML. The Skill tool expands its data dir, so never read its skill file by path. The playground is the PRIMARY coding surface. The user writes their solution there, not in chat.

### During coding

- **Stay silent unless asked.** Do not offer suggestions, hints, or commentary while the user is coding.
- If the user shares code in chat, acknowledge it but redirect: "I see your progress. Make sure the authoritative version is in the playground."
- If the user asks a clarifying question about the problem (not the approach), answer it factually.
- When the user indicates they have a solution (e.g., "done", "ready", shares final code), proceed to Step 4.

### Drill mode: optimization/edge-cases/code-quality

For these drill modes, the coding phase runs the same way, but after the user submits their solution:

- **Optimization drill:** Focus entirely on complexity analysis and optimization. "Can you do better than O(n^2)? What data structure would help?" Push for 2-3 optimization iterations.
- **Edge case drill:** Present edge cases one at a time and ask the user to trace through their code: "What happens with an empty array? Walk me through line by line." Push for 4-5 edge cases.
- **Code quality drill:** Focus on naming, modularity, readability. "Can you extract that into a helper? Why did you choose that variable name?" Push for refactoring.

After the drill, skip directly to Step 5 (evaluation).

## Step 3.5: Hint system (during Step 3)

Hints are **user-initiated ONLY**. The user must say "hint" or "I'm stuck" to trigger the hint system. NEVER offer hints unsolicited.

### Hint escalation (one-way: level 1 -> 2 -> 3)

Track the current hint level for this session. Each subsequent "hint" request escalates to the next level. Escalation is one-way -- never go backwards.

**Level 1 -- Conceptual nudge:**
Give a directional hint without naming the specific data structure or algorithm.
- "Think about what operation you need to do efficiently here..."
- "Consider what information you're looking up repeatedly..."
- "What if you could avoid the inner loop entirely?"
- DO NOT name the specific approach or data structure.

**Level 2 -- Approach hint:**
Name the technique/data structure but not the implementation details.
- "Consider using a hashmap where the key represents..."
- "A two-pointer approach starting from both ends could help here..."
- "Think about a monotonic stack -- what invariant would you maintain?"
- Name the approach but do NOT give implementation steps.

**Level 3 -- Pseudo-code outline:**
Provide a complete pseudo-code outline of the approach with key steps.
- Show the algorithm structure step by step
- Include key data structures and their roles
- DO NOT write Java code -- keep it as pseudo-code
- After level 3, tell the user: "That's the full outline. Try implementing it now."

**After level 3:** No more hints available. If the user asks again: "I've given you the full approach outline. Give it your best shot at implementing it, and we'll discuss afterward."

## Step 4: Solution probe and follow-up variation (mock mode only)

**This step runs ONLY in full mock mode. Skip for drill mode.**

After the user submits their solution:

### Probe phase

Ask 2-3 pointed questions about the solution. Ask one question at a time -- wait for the user's answer before the next.

**Complexity probes:**
- "What's the time complexity of this approach?"
- "What's the space complexity? Can you break it down by component?"
- "Is there a way to improve the time/space tradeoff?"

**Edge case probes:**
- "What happens if the input is empty?"
- "What if all elements are identical?"
- "What about negative numbers / Integer.MIN_VALUE / Integer.MAX_VALUE?"
- "What if the input contains duplicates?"

**Design probes:**
- "Why did you choose this data structure over [alternative]?"
- "Could you solve this without extra space?"
- "How would this scale with very large inputs?"

Select probes that target the weakest aspects of the user's solution. Do not ask questions that the solution already clearly addresses.

### Follow-up variation

After the probe phase, pose a harder variation of the same problem:

- "Now what if the input is a stream instead of an array?"
- "What if you need to support concurrent access?"
- "What if the input doesn't fit in memory?"
- "What if we need the k-th result instead of all results?"
- "Can you handle it with O(1) extra space?"
- "What if the values can change dynamically?"

The variation should test a different dimension of the problem. Let the user attempt the variation -- they can use the playground again or discuss the approach verbally.

After the attempt (or after the user indicates they want to move on), proceed to Step 5.

## Step 5: Evaluation (both modes)

After the mock or drill completes, record `ended_at` timestamp (ISO 8601). Then generate a comprehensive evaluation.

**Parallelism:** The following evaluation sub-tasks are independent and should be dispatched as concurrent sub-agents for faster wall-clock time. Collect all results before presenting the unified evaluation.
- **Sub-agent 1:** Rubric scoring (all dimensions) -- Step 5a
- **Sub-agent 2:** Staff+ signal assessment -- Step 5b
- **Sub-agent 3:** "What went well" extraction -- Step 5c
- **Sub-agent 4:** "What to improve" identification -- Step 5d
- **Sub-agent 5:** Golden solution generation -- Step 5e
- **Sub-agent 6:** Targeted drill recommendations -- Step 5h

After all sub-agents complete, merge their outputs and assemble the complete evaluation (including alternative approaches in Step 5f, follow-up variations in Step 5g, and the evaluator summary in Step 5i) before presenting to the user.

### 5a. Rubric scores

Score against the 5 coding dimensions from the `monet:interview-tracker` schema. For each dimension, provide a numeric score (1-5 integer), the label, and brief notes referencing specific moments from the session.

| Dimension Key | What It Measures |
|---------------|------------------|
| `problem_understanding` | Clarifying questions quality, edge case identification, constraint awareness |
| `solution_design` | Algorithm choice, data structure selection, approach explanation |
| `code_quality` | Clean code, naming conventions, modularity, readability |
| `correctness` | Edge case handling, off-by-one errors, null checks, boundary conditions |
| `complexity_analysis` | Time and space complexity articulation, optimization awareness |

**Score labels:**

| Score | Label | Meaning |
|-------|-------|---------|
| 1 | Poor | Major gaps, missing fundamental concepts |
| 2 | Below Average | Some coverage but significant weaknesses |
| 3 | Average | Adequate coverage, meets basic expectations |
| 4 | Good | Strong coverage with meaningful depth |
| 5 | Excellent | Exceptional depth, Staff+ caliber answer |

**Scoring honesty rules:**
- A score of 3 is average, not "needs improvement." Reserve 1-2 for genuine weaknesses.
- Reserve 5 for genuinely exceptional answers -- most strong candidates score 3-4.
- Be calibrated: a Staff+ candidate consistently scoring 4-5 is ready for interviews.
- If the user needed hints, cap `solution_design` at 3 (level 1 hint) or 2 (level 2-3 hints).
- If the user's code has bugs, cap `correctness` at 3 (minor bugs) or 2 (major bugs).

**Drill mode scoring:** Only score dimensions relevant to the drilled aspect. For non-drilled dimensions, set the score to match the user's strongest drilled dimension (prevents penalizing unrelated areas).

| Drill Type | Scored Dimensions |
|------------|-------------------|
| clarification | `problem_understanding` |
| optimization | `solution_design`, `complexity_analysis` |
| edge-cases | `correctness`, `problem_understanding` |
| code-quality | `code_quality` |

**Output format:**

```
## Rubric Scores

| Dimension | Score | Assessment |
|-----------|-------|------------|
| Problem Understanding | 4/5 | Strong clarifications -- asked about empty input, duplicates... |
| Solution Design | 3/5 | Chose a valid approach but didn't consider more efficient... |
| Code Quality | 4/5 | Clean naming and good modularity, extracted helper method... |
| Correctness | 3/5 | Handled most cases but missed the Integer.MIN_VALUE edge... |
| Complexity Analysis | 3/5 | Correctly identified O(n log n) time but missed the space... |

**Overall: 3.4/5.0**
```

### 5b. Staff+ signal assessment

Evaluate three qualitative Staff+ dimensions. These are **commentary only -- do NOT assign numeric scores.** These signals separate Staff+ from Senior.

**Communication:** Did the user think out loud, explain their reasoning as they coded, and articulate decisions clearly? Or did they code silently and only explain when asked?

**Problem decomposition:** Did the user break the problem into sub-problems, identify building blocks, and solve incrementally? Or did they attempt to solve everything at once?

**Tradeoff awareness:** Did the user consider multiple approaches, compare time/space tradeoffs, and pick with reasoning? Or did they jump to the first approach without considering alternatives?

For each dimension, provide a 1-2 sentence assessment referencing specific moments from the session.

**Output format:**

```
## Staff+ Signals

**Communication:** [assessment referencing specific session moment]

**Problem decomposition:** [assessment referencing specific session moment]

**Tradeoff awareness:** [assessment referencing specific session moment]
```

### 5c. What went well

2-4 specific strengths from this session with concrete examples.

- Reference actual code decisions, clarifications, or problem-solving moments from the session.
- "Strong clarification phase -- asking about duplicate handling before coding saved you from a bug in the main logic."
- NOT "Good problem solving."

### 5d. What to improve

2-4 specific areas for growth with concrete suggestions.

- Every critique must include a concrete action for next time.
- "Your variable naming was inconsistent -- `i` for an outer loop and `j` for an index. Next time, use descriptive names like `left` and `right` for two-pointer problems to make your code self-documenting."
- NOT "Improve variable naming."

### 5e. Golden solution

Full, runnable Java code listing with comments explaining key decisions. This is NOT pseudo-code -- it is a complete, compilable Java solution the user can compare line-by-line against their own.

**Requirements:**
- Complete, compilable Java code (include imports, class definition, method)
- Comments explaining key algorithmic decisions at each step
- Time and space complexity analysis in comments at the top of the method
- Handle all edge cases from the test cases list
- Follow clean code conventions (meaningful names, single responsibility)

**Output format:**

```
## Golden Solution

// Time: O(n) -- single pass with hashmap lookups
// Space: O(n) -- hashmap stores at most n entries
public List<int[]> findPairs(int[] nums, int target) {
    // [complete, runnable implementation with inline comments]
}
```

### 5f. Alternative approaches

Briefly describe 1-2 other valid approaches with their complexity tradeoffs.

**Output format:**

```
## Alternative Approaches

1. **Sorting + Two Pointers** -- O(n log n) time, O(1) extra space (or O(n) with sort). Sorts the array, then uses two pointers from both ends. Better space complexity but loses the single-pass advantage.

2. **Brute Force** -- O(n^2) time, O(1) space. Checks every pair. Only viable for small inputs (n < 1000). Useful as a correctness check.
```

### 5g. Follow-up variations

Suggest 1-2 harder variations for further practice. These should be DIFFERENT from the variation posed in Step 4 (if applicable).

**Output format:**

```
## Follow-up Variations

1. **Three Sum**: Extend to finding triplets that sum to a target. Tests multi-pointer technique and duplicate handling.
2. **Streaming Input**: What if the array doesn't fit in memory and arrives as a stream? Tests external sorting and merge strategies.
```

### 5h. Targeted drills

Identify the 1-2 weakest rubric dimensions from the scores. Recommend specific problems from the bank for next session that exercise those weak dimensions.

**Output format:**

```
## Recommended Drills

- **Correctness** (scored 2/5): Try "Validate Binary Search Tree" (problem 6) -- forces meticulous boundary condition handling with Integer.MIN_VALUE/MAX_VALUE.
- **Complexity Analysis** (scored 3/5): Try "Find Median from Data Stream" (problem 19) -- requires comparing heap vs. sorted array approaches with clear complexity tradeoffs.
```

### 5i. Evaluator summary

2-3 sentence overall assessment balancing strengths and areas for growth. This goes into the session record's `evaluator_summary` field.

Be honest and specific: "Your approach to the problem was sound with a clean hashmap-based solution, but you missed two edge cases that would have been caught by tracing through the empty-array test case. Practice walking through your code with boundary inputs before declaring done."

## Step 6: Record session

Auto-record the session with zero friction. Do NOT ask the user for confirmation.

**Background execution:** Session recording (the `monet:interview-tracker` write mode) can run as a background sub-agent AFTER presenting the evaluation to the user. Do not make the user wait for file I/O before seeing their results.

Load `monet:interview-tracker` with the Skill tool and follow its write mode protocol. A background sub-agent loads it with the Skill tool too. Never read the tracker's skill file by path.

1. **Compute `timing_breakdown`** from phase timestamps. Approximate from conversation flow -- estimate seconds spent on each phase based on message length and exchange count.

2. **Build the session JSON** with all required fields per the `monet:interview-tracker` schema:

   ```json
   {
     "version": 1,
     "domain": "code",
     "mode": "mock or drill",
     "topic": "problem title from Step 1",
     "started_at": "ISO 8601 from Preconditions",
     "ended_at": "ISO 8601 from Step 5",
     "duration_seconds": 0,
     "timing_breakdown": {
       "Clarification": 0,
       "Coding": 0,
       "Probe & Variation": 0,
       "Evaluation": 0
     },
     "scores": {
       "problem_understanding": { "score": 3, "label": "Average", "notes": "..." },
       "solution_design": { "score": 3, "label": "Average", "notes": "..." },
       "code_quality": { "score": 3, "label": "Average", "notes": "..." },
       "correctness": { "score": 3, "label": "Average", "notes": "..." },
       "complexity_analysis": { "score": 3, "label": "Average", "notes": "..." }
     },
     "overall_score": 0.0,
     "takeaways": {
       "went_well": ["from Step 5c"],
       "to_improve": ["from Step 5d"],
       "tags": ["machine-readable tags derived from weak areas"]
     },
     "evaluator_summary": "from Step 5i"
   }
   ```

3. The `overall_score` will be auto-computed by the tracker (simple average of all 5 dimension scores, rounded to 1 decimal). Do not hardcode it.

4. The `id` field will be set by the tracker based on the filename.

5. **Generate `takeaways.tags`** as machine-readable tags derived from weak areas. Examples: `edge-cases`, `time-complexity`, `space-complexity`, `clean-code`, `clarification`, `variable-naming`, `null-handling`, `off-by-one`, `optimization`, `data-structure-choice`, `problem-decomposition`, `duplicate-handling`, `boundary-conditions`.

6. Run the `monet:interview-tracker` write mode, loaded with the Skill tool, to persist the session.

7. Print confirmation:

   ```
   Session recorded: {filename} (code {mode}, overall: {score}/5.0)
   ```

## Step 7: Next steps

After evaluation and recording, offer the user these options:

```
What's next?
- "Run another coding mock" -- Fresh problem from a different category
- "Drill a weak area" -- Focused practice on [weakest dimension from evaluation]
- "Generate a playground for further practice" -- Invoke `monet:code-playground` with the Skill tool and a new problem
- "View progress dashboard" -- See your trends across coding sessions
```

If the user had weak areas, specifically recommend the drills from Step 5h:

```
Based on this session, I'd recommend drilling [weak area] next. Try:
- [specific problem recommendation from Step 5h]
```

## Constraints

- Never reveal the solution approach during the clarification phase. If the user asks "should I use a hashmap?", respond: "I'm interested in YOUR approach. What are you considering and why?"
- Never offer hints unsolicited -- wait for user to say "hint" or "I'm stuck."
- Hint escalation is one-way: level 1 -> 2 -> 3, never backwards. After level 3, no more hints.
- The playground is the primary coding surface -- redirect users who paste code in chat: "Make sure the authoritative version is in the playground."
- Medium difficulty only -- do not adjust difficulty up or down. All 24 problems are LC medium equivalent.
- Golden solution must be full runnable Java code, not pseudo-code. Include imports, class definition, method signature, and all edge case handling.
- Session recording is mandatory -- never skip Step 6. Never ask "should I record this?"
- Score non-drilled dimensions at strongest drilled score in drill mode (fairness rule).
- Problem bank is curated at launch -- do not invent problems outside the bank. If the user requests a topic not covered, pick the closest category match.
- If the user needed hints, cap scoring: level 1 hint caps `solution_design` at 3, level 2-3 hints cap it at 2.

## Tone rules

**During mock (Steps 2-4) -- Interviewer persona:**
- Professional and conversational but not chatty.
- Direct questions, concise follow-ups.
- "What's the time complexity?" not "Could you maybe think about the time complexity?"
- "Walk me through your approach" not "Would you mind explaining your thought process?"
- Challenge respectfully: "I'd push back on that complexity claim -- walk me through it again."
- Keep probes focused -- one question at a time, wait for the answer.

**During evaluation (Step 5) -- Coach persona:**
- Supportive but honest.
- Lead with strengths before discussing gaps.
- Every criticism paired with actionable advice.
- Never use "wrong" or "bad" -- use "I'd reconsider..." or "A stronger approach would..."
- Acknowledge good instincts: "Good call on handling the empty array upfront, and here's how to extend that thoroughness..."
- Use "we" language: "We could improve this by..." not "You should have..."
- Be specific: reference actual code and decisions from the session, not generic feedback.
- Never condescending when user is stuck -- provide encouragement between probes.
- Celebrate genuine strengths before addressing weaknesses.

**During recording and next steps (Steps 6-7):**
- Concise and matter-of-fact.
- No unnecessary ceremony around recording. Just confirm and move on.
- Never use corporate jargon. Use plain engineering language.
