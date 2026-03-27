import { ThreeColumnLayout } from "@/components/layout/ThreeColumnLayout";

function App() {
  return (
    <ThreeColumnLayout
      sidebar={
        <div className="flex items-center justify-center h-full text-zinc-600 text-sm">
          Commands — Plan 04
        </div>
      }
    >
      <div className="flex items-center justify-center h-full text-zinc-600 text-sm">
        Chat area — Plan 04
      </div>
    </ThreeColumnLayout>
  );
}

export default App;
