import { ThreeColumnLayout } from "@/components/layout/ThreeColumnLayout";
import { ChatPane } from "@/components/chat/ChatPane";
import { CommandSidebar } from "@/components/sidebar/CommandSidebar";

function App() {
  return (
    <ThreeColumnLayout sidebar={<CommandSidebar />}>
      <ChatPane />
    </ThreeColumnLayout>
  );
}

export default App;
