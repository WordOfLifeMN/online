Code for searching: https://g.co/gemini/share/5c0629f5bddd

```
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Search & Highlight</title>
    <!-- Use Tailwind CSS for modern styling -->
    <script src="https://cdn.tailwindcss.com"></script>
    <style>
        /* Custom CSS for the highlight effect */
        .highlight {
            background-color: #fde047; /* Tailwind's yellow-300 */
            font-weight: bold;
            border-radius: 4px;
            padding: 2px 4px;
        }

        /* Additional custom styles */
        body {
            font-family: 'Inter', sans-serif;
            background-color: #f3f4f6;
        }
    </style>
</head>
<body class="p-8">

    <!-- Search Bar and Controls -->
    <div class="fixed top-0 left-0 right-0 bg-white shadow-md p-4 flex flex-col sm:flex-row items-center justify-center space-y-2 sm:space-y-0 sm:space-x-4 z-50">
        <input type="text" id="searchInput" placeholder="Enter text to search..." class="flex-grow p-3 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-yellow-500 transition-all">
        <button id="searchButton" class="w-full sm:w-auto px-6 py-3 bg-yellow-500 text-white font-bold rounded-lg hover:bg-yellow-600 transition-colors shadow-md">
            Search
        </button>
        <div id="resultsMessage" class="text-sm font-medium text-gray-600 mt-2 sm:mt-0"></div>
    </div>

    <!-- Padding for the fixed search bar -->
    <div class="h-20"></div>

    <!-- Main content container with multiple divs -->
    <div class="space-y-8">
        <div class="bg-white p-6 rounded-xl shadow-lg border border-gray-200">
            <h2 class="text-2xl font-bold mb-2 text-gray-800">Introduction to the Universe</h2>
            <p class="text-gray-700 leading-relaxed">
                The universe is a vast expanse of space and time, encompassing all matter, energy, planets, stars, galaxies, and the contents of intergalactic space. It is a concept that has fascinated humanity for centuries. Our understanding of the universe has evolved dramatically, from ancient geocentric models to the modern Big Bang theory. Studying the universe helps us answer fundamental questions about our existence and place in the cosmos. The search for extraterrestrial life and dark matter continues to push the boundaries of science.
            </p>
        </div>

        <div class="bg-white p-6 rounded-xl shadow-lg border border-gray-200">
            <h2 class="text-2xl font-bold mb-2 text-gray-800">The Power of the Internet</h2>
            <p class="text-gray-700 leading-relaxed">
                The internet is a global network of interconnected computer systems, providing a platform for communication, information sharing, and commerce. It has transformed nearly every aspect of modern life, from how we work and learn to how we socialize and entertain ourselves. The internet's impact on society is profound, enabling instant global communication and access to an unprecedented amount of knowledge. The search for new applications and technologies within this domain is constant.
            </p>
        </div>

        <div class="bg-white p-6 rounded-xl shadow-lg border border-gray-200">
            <h2 class="text-2xl font-bold mb-2 text-gray-800">Sustainable Agriculture</h2>
            <p class="text-gray-700 leading-relaxed">
                Sustainable agriculture is a method of farming that is environmentally sound, economically viable, and socially responsible. It focuses on practices that protect the environment, conserve natural resources, and ensure the long-term productivity of the land. Key principles include crop rotation, integrated pest management, and reduced reliance on synthetic fertilizers. A global effort to find sustainable solutions is a priority.
            </p>
        </div>

        <div class="bg-white p-6 rounded-xl shadow-lg border border-gray-200">
            <h2 class="text-2xl font-bold mb-2 text-gray-800">The Human Brain</h2>
            <p class="text-gray-700 leading-relaxed">
                The human brain is an incredibly complex organ, serving as the command center for the nervous system. It controls thought, memory, emotion, touch, motor skills, vision, breathing, temperature, and every process that regulates our body. The brain is composed of billions of neurons, which communicate with each other through electrical and chemical signals. Research into its functions and diseases is a critical area of scientific inquiry. The search for a deeper understanding of the brain's complexities continues.
            </p>
        </div>

    </div>

    <script>
        // Get references to the DOM elements
        const searchInput = document.getElementById('searchInput');
        const searchButton = document.getElementById('searchButton');
        const resultsMessage = document.getElementById('resultsMessage');
        const contentDivs = document.querySelectorAll('.space-y-8 > div');
        let originalContent = new Map(); // Use a Map to store original content for resetting

        // Store the original innerHTML of each content div
        contentDivs.forEach((div, index) => {
            originalContent.set(div, div.innerHTML);
        });

        // Function to remove all existing highlights
        const removeHighlights = () => {
            document.querySelectorAll('.highlight').forEach(span => {
                // Replace the span with its text content to "un-highlight"
                const parent = span.parentNode;
                if (parent) {
                    parent.replaceChild(document.createTextNode(span.textContent), span);
                    // Normalize the parent node to merge adjacent text nodes
                    parent.normalize();
                }
            });
        };

        // Main search and highlight function
        const searchAndHighlight = () => {
            // First, reset all content to its original state
            contentDivs.forEach(div => {
                if (originalContent.has(div)) {
                    div.innerHTML = originalContent.get(div);
                }
            });
            resultsMessage.textContent = ''; // Clear previous message

            const query = searchInput.value.trim();
            if (!query) {
                return; // Do nothing if the search query is empty
            }

            let foundCount = 0;
            let firstMatch = null;

            // Use a case-insensitive regular expression for searching
            const regex = new RegExp(query, 'gi');

            contentDivs.forEach(div => {
                const textContent = div.textContent;
                // Check if the query exists in the text content
                if (textContent.toLowerCase().includes(query.toLowerCase())) {
                    // Re-read the original HTML content to avoid issues with previous highlights
                    const originalHtml = originalContent.get(div);
                    const newHtml = originalHtml.replace(regex, `<span class="highlight">$&</span>`);
                    div.innerHTML = newHtml;
                    
                    // Count all occurrences in the newly updated HTML
                    const matchesInDiv = (newHtml.match(/<span class="highlight">/g) || []).length;
                    foundCount += matchesInDiv;

                    // Find the first highlight span to scroll to
                    if (!firstMatch) {
                        firstMatch = div.querySelector('.highlight');
                    }
                }
            });

            // Display results and scroll to the first match if found
            if (foundCount > 0) {
                resultsMessage.textContent = `Found ${foundCount} matches.`;
                if (firstMatch) {
                    firstMatch.scrollIntoView({ behavior: 'smooth', block: 'center' });
                }
            } else {
                resultsMessage.textContent = 'No matches found.';
            }
        };

        // Attach event listeners
        searchButton.addEventListener('click', searchAndHighlight);
        searchInput.addEventListener('keydown', (event) => {
            if (event.key === 'Enter') {
                searchAndHighlight();
            }
        });

    </script>
</body>
</html>
```